package orchestrator

import (
	"context"
	"fmt"
	"net"

	"github.com/render-oss/cli/pkg/workflows/store"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
)

type TaskNotFoundError struct {
	TaskIdentifier string
}

func (e *TaskNotFoundError) Error() string {
	return fmt.Sprintf("task not found: %s", e.TaskIdentifier)
}

type StatusReporter interface {
	TaskEnqueued(taskRun *store.TaskRun)
	TaskRunning(taskRun *store.TaskRun)
	TaskCompleted(taskRun *store.TaskRun)
	TaskFailed(taskRun *store.TaskRun)
}

type Coordinator struct {
	store   *store.TaskStore
	sdkExec sdkExec

	callbackURL string

	socketTracker *SocketTracker
	serverFactory serverFactory

	topLevelContext context.Context
	statusReporter  StatusReporter
}

type sdkExec interface {
	StartService(ctx context.Context, taskRunID string, socket string, mode Mode) (CleanupFunc, error)
}

type serverFactory interface {
	NewHandler(
		socket net.Listener,
		input taskserver.GetInput200JSONResponse,
		getSubtaskResultFunc taskserver.GetSubtaskResultFunc,
		startSubtaskFunc taskserver.StartSubtaskFunc,
	) *taskserver.ServerHandler
}

func NewCoordinator(ctx context.Context, store *store.TaskStore, sdkExec sdkExec, socketTracker *SocketTracker, serverFactory serverFactory, statusReporter StatusReporter) *Coordinator {
	coordinator := &Coordinator{
		store:           store,
		sdkExec:         sdkExec,
		socketTracker:   socketTracker,
		serverFactory:   serverFactory,
		statusReporter:  statusReporter,
		topLevelContext: ctx,
	}

	return coordinator
}

func (c *Coordinator) GetSubtaskResult(taskRunID string) (taskserver.PostGetSubtaskResultResponseObject, error) {
	taskRun := c.store.GetTaskRun(taskRunID)
	if taskRun == nil {
		return taskserver.PostGetSubtaskResult500Response{}, nil
	}

	var complete *taskserver.TaskComplete
	var taskError *taskserver.TaskError
	if taskRun.Status == store.TaskRunStatusComplete {
		complete = &taskserver.TaskComplete{
			Output: taskRun.Output,
		}
	}
	if taskRun.Status == store.TaskRunStatusFailed {
		taskError = &taskserver.TaskError{
			Details: *taskRun.Error,
		}
	}

	return taskserver.PostGetSubtaskResult200JSONResponse{
		Complete:     complete,
		StillRunning: taskRun.Status == store.TaskRunStatusRunning,
		Error:        taskError,
	}, nil
}

func (c *Coordinator) StartSubtaskFunc(parentTaskRunID string) taskserver.StartSubtaskFunc {
	return func(taskName string, input []byte) (taskserver.PostRunSubtaskResponseObject, error) {
		task := c.store.GetTaskByName(taskName)
		if task == nil {
			return taskserver.PostRunSubtask500Response{}, fmt.Errorf("subtask \"%s\" not found", taskName)
		}
		taskRun, err := c.StartTask(c.topLevelContext, task.ID, input, &parentTaskRunID)
		if err != nil {
			return taskserver.PostRunSubtask500Response{}, fmt.Errorf("error running subtask: %w", err)
		}

		return taskserver.PostRunSubtask200JSONResponse{
			TaskRunId: taskRun.ID,
		}, nil
	}
}

func (c *Coordinator) StartTask(ctx context.Context, taskIdentifier string, input []byte, parentTaskRunID *string) (*store.TaskRun, error) {
	// Ensure tasks are up to date
	if _, err := c.PopulateTasks(ctx); err != nil {
		return nil, err
	}

	task := c.store.GetTask(taskIdentifier)
	if task == nil {
		return nil, &TaskNotFoundError{TaskIdentifier: taskIdentifier}
	}

	taskName := task.Name

	socket, err := c.socketTracker.NewSocket()
	if err != nil {
		return nil, err
	}

	taskRun := c.store.StartTaskRun(taskName, input, parentTaskRunID)

	c.statusReporter.TaskEnqueued(taskRun)

	server := c.serverFactory.NewHandler(socket, taskserver.GetInput200JSONResponse{
		TaskName: taskName,
		Input:    input,
	}, c.GetSubtaskResult, c.StartSubtaskFunc(taskRun.ID))

	server.Start()

	// Pass in a background context to avoid
	cleanupFunc, err := c.sdkExec.StartService(context.Background(), taskRun.ID, socket.Addr().String(), ModeRun)
	if err != nil {
		return nil, err
	}

	c.statusReporter.TaskRunning(taskRun)

	go func() {
		defer cleanupFunc()

		callback := <-server.Channels.PostCallback

		err := c.completeTask(callback.Body, taskRun.ID)
		if err != nil {
			fmt.Println("error completing task", err)
		}
	}()

	return taskRun, nil
}

func (c *Coordinator) PopulateTasks(ctx context.Context) ([]*store.Task, error) {
	socket, err := c.socketTracker.NewSocket()
	if err != nil {
		return nil, err
	}

	server := c.serverFactory.NewHandler(socket, taskserver.GetInput200JSONResponse{}, c.GetSubtaskResult, c.StartSubtaskFunc(""))

	server.Start()

	// we don't need to pass in a task run id here, because we don't need to
	// keep track of the logs for registration
	cleanupFunc, err := c.sdkExec.StartService(context.Background(), "", socket.Addr().String(), ModeRegister)
	if err != nil {
		return nil, err
	}
	defer cleanupFunc()

	// Wait until either the context is done or the server has received the tasks
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case tasks := <-server.Channels.PostTasks:
		c.store.SetTasks(tasks.Body.Tasks)
		return c.store.GetTasks(), nil
	}
}

func (c *Coordinator) completeTask(completeBody *taskserver.CallbackRequest, taskRunID string) error {
	taskRun := c.store.GetTaskRun(taskRunID)
	if taskRun == nil {
		return fmt.Errorf("task run not found")
	}

	if completeBody.Complete != nil {
		updated, err := c.store.CompleteTaskRun(taskRun.ID, completeBody.Complete.Output)
		if err != nil {
			return err
		}
		c.statusReporter.TaskCompleted(updated)
	} else if completeBody.Error != nil {
		updated, err := c.store.FailTaskRun(taskRun.ID, completeBody.Error.Details)
		if err != nil {
			return err
		}
		c.statusReporter.TaskFailed(updated)
	}

	return nil
}

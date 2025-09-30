package orchestrator

import (
	"context"
	"fmt"
	"net"

	"github.com/render-oss/cli/pkg/workflows/store"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
)

const (
	minPort = 10000
	maxPort = 20000
)

type TaskNotFoundError struct {
	TaskIdentifier string
}

func (e *TaskNotFoundError) Error() string {
	return fmt.Sprintf("task not found: %s", e.TaskIdentifier)
}

type Coordinator struct {
	store   *store.TaskStore
	sdkExec sdkExec

	callbackURL string

	socketTracker *SocketTracker
	serverFactory serverFactory

	topLevelContext context.Context
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

func NewCoordinator(ctx context.Context, store *store.TaskStore, sdkExec sdkExec, socketTracker *SocketTracker, serverFactory serverFactory) *Coordinator {
	return &Coordinator{
		store:           store,
		sdkExec:         sdkExec,
		socketTracker:   socketTracker,
		serverFactory:   serverFactory,
		topLevelContext: ctx,
	}
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
		StillRunning: taskRun.Status != store.TaskRunStatusComplete,
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
	_, err := c.PopulateTasks(ctx)
	if err != nil {
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

	go func() {
		defer cleanupFunc()

		callback := <-server.Channels.PostCallback

		err := c.completeTask(c.topLevelContext, callback.Body, taskRun.ID)
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

func (c *Coordinator) completeTask(ctx context.Context, completeBody *taskserver.CallbackRequest, taskRunID string) error {
	taskRun := c.store.GetTaskRun(taskRunID)
	if taskRun == nil {
		return fmt.Errorf("task run not found")
	}

	if completeBody.Complete != nil {
		c.store.CompleteTaskRun(taskRun.ID, completeBody.Complete.Output)
	} else if completeBody.Error != nil {
		c.store.FailTaskRun(taskRun.ID, completeBody.Error.Details)
	}

	return nil
}

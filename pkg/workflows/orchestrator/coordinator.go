package orchestrator

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/render-oss/cli/pkg/workflows/store"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
)

type TaskNotFoundError struct {
	TaskSlug  string
	TaskRunID string
}

func (e *TaskNotFoundError) Error() string {
	if e.TaskRunID != "" {
		return fmt.Sprintf("task run not found: %s", e.TaskRunID)
	}
	return fmt.Sprintf("task not found: %s", e.TaskSlug)
}

type StatusReporter interface {
	TaskEnqueued(taskRun *store.TaskRun)
	TaskRunning(taskRun *store.TaskRun)
	TaskCompleted(taskRun *store.TaskRun)
	TaskFailed(taskRun *store.TaskRun)
	TaskCancelled(taskRun *store.TaskRun)
	TaskNotFound(taskSlug string)
}

type Coordinator struct {
	store   *store.TaskStore
	sdkExec sdkExec

	callbackURL string

	socketTracker *SocketTracker
	serverFactory serverFactory

	topLevelContext context.Context
	statusReporter  StatusReporter

	activeRuns   map[string]*activeRunEntry
	activeRunsMu sync.Mutex
}

type activeRunEntry struct {
	cancel        context.CancelFunc
	rootTaskRunID string
}

type sdkExec interface {
	StartService(ctx context.Context, taskRunID string, socket string, mode Mode) (CleanupFunc, <-chan error, error)
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
		activeRuns:      make(map[string]*activeRunEntry),
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

// StartSubtaskFunc builds the taskserver.StartSubtaskFunc callback that
// the in-process task server invokes when a running task spawns a subtask.
// parent is the TaskRun doing the spawning; pass nil when no subtasks are
// expected (e.g. during task registration).
func (c *Coordinator) StartSubtaskFunc(parent *store.TaskRun) taskserver.StartSubtaskFunc {
	return func(taskName string, input []byte) (taskserver.PostRunSubtaskResponseObject, error) {
		task := c.store.GetTaskByName(taskName)
		if task == nil {
			return taskserver.PostRunSubtask500Response{}, fmt.Errorf("subtask \"%s\" not found", taskName)
		}
		taskRun, err := c.StartTask(c.topLevelContext, task.ID, input, parent)
		if err != nil {
			return taskserver.PostRunSubtask500Response{}, fmt.Errorf("error running subtask: %w", err)
		}

		return taskserver.PostRunSubtask200JSONResponse{
			TaskRunId: taskRun.ID,
		}, nil
	}
}

func (c *Coordinator) StartTask(ctx context.Context, taskSlug string, input []byte, parent *store.TaskRun) (*store.TaskRun, error) {
	// Ensure tasks are up to date
	if _, err := c.PopulateTasks(ctx); err != nil {
		return nil, err
	}

	task := c.store.GetTask(taskSlug)
	if task == nil {
		c.statusReporter.TaskNotFound(taskSlug)
		return nil, &TaskNotFoundError{TaskSlug: taskSlug}
	}

	taskName := task.Name

	socket, err := c.socketTracker.NewSocket()
	if err != nil {
		return nil, err
	}

	c.activeRunsMu.Lock()
	var parentTaskRunID *string
	if parent != nil {
		// Subtasks are gated on the root task run being active
		if _, ok := c.activeRuns[parent.RootTaskRunID]; !ok {
			// If the root task run is not found, that implies it's been canceled
			c.activeRunsMu.Unlock()
			return nil, fmt.Errorf("root task run %s is not active (canceled or completed)", parent.RootTaskRunID)
		}
		parentTaskRunID = &parent.ID
	}

	runCtx, cancelRun := context.WithCancel(context.Background())
	taskRun := c.store.StartTaskRun(taskName, input, parentTaskRunID)
	c.activeRuns[taskRun.ID] = &activeRunEntry{
		cancel:        cancelRun,
		rootTaskRunID: taskRun.RootTaskRunID,
	}
	c.activeRunsMu.Unlock()

	c.statusReporter.TaskEnqueued(taskRun)

	server := c.serverFactory.NewHandler(socket, taskserver.GetInput200JSONResponse{
		TaskName: taskName,
		Input:    input,
	}, c.GetSubtaskResult, c.StartSubtaskFunc(taskRun))

	server.Start()

	cleanupFunc, processDone, err := c.sdkExec.StartService(runCtx, taskRun.ID, socket.Addr().String(), ModeRun)
	if err != nil {
		cancelRun()
		errMsg := fmt.Sprintf("failed to start task: %s", err)
		if updated, failErr := c.store.FailTaskRun(taskRun.ID, errMsg); failErr != nil {
			fmt.Println("error failing task", failErr)
		} else {
			c.statusReporter.TaskFailed(updated)
		}

		c.activeRunsMu.Lock()
		delete(c.activeRuns, taskRun.ID)
		c.activeRunsMu.Unlock()
		return nil, err
	}

	c.statusReporter.TaskRunning(taskRun)

	go func() {
		defer cleanupFunc()
		defer func() {
			c.activeRunsMu.Lock()
			delete(c.activeRuns, taskRun.ID)
			c.activeRunsMu.Unlock()
			cancelRun()
		}()

		select {
		case callback := <-server.Channels.PostCallback:
			err := c.completeTask(callback.Body, taskRun.ID)
			if err != nil {
				fmt.Println("error completing task", err)
			}
		case err := <-processDone:
			// If a cancellation callsite has already transitioned this task
			// run to a terminal state, don't overwrite it with a failure.
			// Cancelling runCtx kills the underlying process, which fires
			// processDone in addition to runCtx.Done().
			if tr := c.store.GetTaskRun(taskRun.ID); tr != nil && tr.Status != store.TaskRunStatusRunning {
				return
			}
			errMsg := "start command exited before completing the task"
			if err != nil {
				errMsg = fmt.Sprintf("%s: %s", errMsg, err)
			}
			updated, failErr := c.store.FailTaskRun(taskRun.ID, errMsg)
			if failErr != nil {
				fmt.Println("error failing task", failErr)
				return
			}
			c.statusReporter.TaskFailed(updated)
		}
	}()

	return taskRun, nil
}

func (c *Coordinator) PopulateTasks(ctx context.Context) ([]*store.Task, error) {
	socket, err := c.socketTracker.NewSocket()
	if err != nil {
		return nil, err
	}

	server := c.serverFactory.NewHandler(socket, taskserver.GetInput200JSONResponse{}, c.GetSubtaskResult, c.StartSubtaskFunc(nil))

	server.Start()

	// we don't need to pass in a task run id here, because we don't need to
	// keep track of the logs for registration
	cleanupFunc, processDone, err := c.sdkExec.StartService(context.Background(), "", socket.Addr().String(), ModeRegister)
	if err != nil {
		return nil, err
	}
	defer cleanupFunc()

	// Wait until either the context is done, the process exits, or the server has received the tasks
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-processDone:
		if err != nil {
			return nil, fmt.Errorf("start command exited before registering tasks: %w\nRun with --debug to see process output", err)
		}
		return nil, fmt.Errorf("start command exited before registering tasks\nRun with --debug to see process output")
	case tasks := <-server.Channels.PostTasks:
		c.store.SetTasks(tasks.Body.Tasks)
		return c.store.GetTasks(), nil
	}
}

// CancelTaskRun cancels a root task run and all of its in-flight descendants.
// Subtasks cannot be canceled directly; cancel the root and the cascade will
// reach them.
func (c *Coordinator) CancelTaskRun(taskRunID string) error {
	taskRun := c.store.GetTaskRun(taskRunID)
	if taskRun == nil {
		return &TaskNotFoundError{TaskRunID: taskRunID}
	}
	if taskRun.ParentTaskRunID != nil {
		return fmt.Errorf("task run %s is a subtask; cancel its root %s instead", taskRunID, taskRun.RootTaskRunID)
	}

	rootID := taskRun.ID

	c.activeRunsMu.Lock()
	defer c.activeRunsMu.Unlock()

	canceled := 0
	for id, entry := range c.activeRuns {
		if entry.rootTaskRunID != rootID {
			continue
		}
		entry.cancel()
		c.markCancelled(id)
		delete(c.activeRuns, id)
		canceled++
	}

	if canceled == 0 {
		return fmt.Errorf("task run %s is not active", taskRunID)
	}
	return nil
}

func (c *Coordinator) markCancelled(taskRunID string) {
	updated, err := c.store.CancelTaskRun(taskRunID)
	if err != nil {
		fmt.Println("error cancelling task", err)
		return
	}
	c.statusReporter.TaskCancelled(updated)
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

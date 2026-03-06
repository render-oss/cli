package orchestrator_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/workflows/orchestrator"
	"github.com/render-oss/cli/pkg/workflows/store"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
	"github.com/stretchr/testify/require"
)

type fakeSdkExec struct {
	startService func(ctx context.Context, taskRunID string, socket string, mode orchestrator.Mode) (cleanupFunc func() error, processDone <-chan error, err error)
}

func (f *fakeSdkExec) StartService(ctx context.Context, taskRunID string, socket string, mode orchestrator.Mode) (orchestrator.CleanupFunc, <-chan error, error) {
	return f.startService(ctx, taskRunID, socket, mode)
}

// neverExits returns a process-done channel that never fires, simulating a long-running process.
func neverExits() <-chan error {
	return make(chan error)
}

type fakeServerFactory struct {
	newHandler func(socket net.Listener, input taskserver.GetInput200JSONResponse, getSubtaskResultFunc taskserver.GetSubtaskResultFunc, startSubtaskFunc taskserver.StartSubtaskFunc) *taskserver.ServerHandler
}

type noopStatusReporter struct{}

func (noopStatusReporter) Ready()                               {}
func (noopStatusReporter) TasksRegistered(taskNames []string)   {}
func (noopStatusReporter) TaskEnqueued(taskRun *store.TaskRun)  {}
func (noopStatusReporter) TaskRunning(taskRun *store.TaskRun)   {}
func (noopStatusReporter) TaskCompleted(taskRun *store.TaskRun) {}
func (noopStatusReporter) TaskFailed(taskRun *store.TaskRun)    {}
func (noopStatusReporter) TaskNotFound(taskSlug string)   {}

func (f *fakeServerFactory) NewHandler(socket net.Listener, input taskserver.GetInput200JSONResponse, getSubtaskResultFunc taskserver.GetSubtaskResultFunc, startSubtaskFunc taskserver.StartSubtaskFunc) *taskserver.ServerHandler {
	return f.newHandler(socket, input, getSubtaskResultFunc, startSubtaskFunc)
}

type recordingStatusReporter struct {
	noopStatusReporter
	notFoundIdentifier string
}

func (r *recordingStatusReporter) TaskNotFound(taskSlug string) {
	r.notFoundIdentifier = taskSlug
}

func TestStartTaskNotFound(t *testing.T) {
	ctx := context.Background()

	s := store.NewTaskStore()

	tasks := []taskserver.Task{
		{
			Name: "test-task",
		},
	}

	postTasksChan := make(chan taskserver.PostRegisterTasksRequestObject, 1)
	postTasksChan <- taskserver.PostRegisterTasksRequestObject{
		Body: &taskserver.PostRegisterTasksJSONRequestBody{
			Tasks: tasks,
		},
	}

	socketTracker, err := orchestrator.NewSocketTracker(ctx)
	require.NoError(t, err)

	reporter := &recordingStatusReporter{}
	coordinator := orchestrator.NewCoordinator(
		ctx,
		s,
		&fakeSdkExec{
			startService: func(ctx context.Context, taskRunID string, socket string, mode orchestrator.Mode) (func() error, <-chan error, error) {
				return func() error { return nil }, neverExits(), nil
			},
		},
		socketTracker,
		&fakeServerFactory{
			newHandler: func(socket net.Listener, input taskserver.GetInput200JSONResponse, getSubtaskResultFunc taskserver.GetSubtaskResultFunc, startSubtaskFunc taskserver.StartSubtaskFunc) *taskserver.ServerHandler {
				return &taskserver.ServerHandler{
					Socket: socket,
					Input:  input,
					Channels: taskserver.ServerChannels{
						PostCallback: make(chan taskserver.PostCallbackRequestObject),
						PostTasks:    postTasksChan,
					},
				}
			},
		},
		reporter,
	)

	_, err = coordinator.StartTask(ctx, "nonexistent-task", []byte{}, nil)
	require.Error(t, err)

	var taskNotFoundErr *orchestrator.TaskNotFoundError
	require.ErrorAs(t, err, &taskNotFoundErr)
	require.Equal(t, "nonexistent-task", taskNotFoundErr.TaskSlug)
	require.Equal(t, "nonexistent-task", reporter.notFoundIdentifier)
}

func TestStartTask(t *testing.T) {
	ctx := context.Background()

	s := store.NewTaskStore()

	tasks := []taskserver.Task{
		{
			Name: "test-task",
		},
	}

	callbackChan := make(chan taskserver.PostCallbackRequestObject)
	postTasksChan := make(chan taskserver.PostRegisterTasksRequestObject, 1)

	postTasksChan <- taskserver.PostRegisterTasksRequestObject{
		Body: &taskserver.PostRegisterTasksJSONRequestBody{
			Tasks: tasks,
		},
	}

	socketTracker, err := orchestrator.NewSocketTracker(ctx)
	if err != nil {
		t.Fatalf("Failed to create socket tracker: %v", err)
	}

	cleanupCalled := false

	cleanupFunc := func() error {
		cleanupCalled = true
		return nil
	}

	startCount := 0
	coordinator := orchestrator.NewCoordinator(
		ctx,
		s,
		&fakeSdkExec{
			startService: func(ctx context.Context, taskRunID string, socket string, mode orchestrator.Mode) (func() error, <-chan error, error) {
				if startCount == 0 {
					require.Equal(t, mode, orchestrator.ModeRegister)
					startCount++
				} else {
					require.Equal(t, mode, orchestrator.ModeRun)
				}

				return cleanupFunc, neverExits(), nil
			},
		},
		socketTracker,
		&fakeServerFactory{
			newHandler: func(socket net.Listener, input taskserver.GetInput200JSONResponse, getSubtaskResultFunc taskserver.GetSubtaskResultFunc, startSubtaskFunc taskserver.StartSubtaskFunc) *taskserver.ServerHandler {
				return &taskserver.ServerHandler{
					Socket: socket,
					Input:  input,
					Channels: taskserver.ServerChannels{
						PostCallback: callbackChan,
						PostTasks:    postTasksChan,
					},
				}
			},
		},
		&noopStatusReporter{},
	)

	_, err = coordinator.StartTask(ctx, "test-task", []byte{}, nil)
	require.NoError(t, err)

	ts := s.GetTasks()
	taskRuns := s.GetTaskRuns(ts[0].ID)

	require.Equal(t, 1, len(taskRuns))
	require.Equal(t, "test-task", taskRuns[0].TaskName)

	callbackChan <- taskserver.PostCallbackRequestObject{
		Body: &taskserver.CallbackRequest{
			Complete: &taskserver.TaskComplete{
				Output: []byte("done"),
			},
		},
	}

	require.Equal(t, store.TaskRunStatusComplete, s.GetTaskRun(taskRuns[0].ID).Status)
	require.Equal(t, json.RawMessage([]byte("done")), s.GetTaskRun(taskRuns[0].ID).Output)

	require.True(t, cleanupCalled)
}

func TestStartTaskWithSubtask(t *testing.T) {
	ctx := context.Background()

	s := store.NewTaskStore()

	tasks := []taskserver.Task{
		{
			Name: "test-task",
		},
		{
			Name: "test-subtask",
		},
	}

	callbackChan := make(chan taskserver.PostCallbackRequestObject)
	postTasksChan := make(chan taskserver.PostRegisterTasksRequestObject, 1)

	subtaskPostCallbackChan := make(chan taskserver.PostCallbackRequestObject)
	subtaskPostTasksChan := make(chan taskserver.PostRegisterTasksRequestObject, 1)

	socketTracker, err := orchestrator.NewSocketTracker(ctx)
	if err != nil {
		t.Fatalf("Failed to create socket tracker: %v", err)
	}

	cleanupFunc := func() error {
		return nil
	}

	subtaskRunID := ""
	var getSubtaskResultFunc taskserver.GetSubtaskResultFunc
	var startSubtaskFunc taskserver.StartSubtaskFunc

	coordinator := orchestrator.NewCoordinator(
		ctx,
		s,
		&fakeSdkExec{
			startService: func(ctx context.Context, taskRunID string, socket string, mode orchestrator.Mode) (func() error, <-chan error, error) {
				return cleanupFunc, neverExits(), nil
			},
		},
		socketTracker,
		&fakeServerFactory{
			newHandler: func(socket net.Listener, input taskserver.GetInput200JSONResponse, getFunc taskserver.GetSubtaskResultFunc, startFunc taskserver.StartSubtaskFunc) *taskserver.ServerHandler {
				if input.TaskName == "test-subtask" {
					return &taskserver.ServerHandler{
						Socket: socket,
						Input:  input,
						Channels: taskserver.ServerChannels{
							PostCallback: subtaskPostCallbackChan,
							PostTasks:    subtaskPostTasksChan,
						},
					}
				}

				if input.TaskName == "test-task" {
					// Set functions for use in the test
					startSubtaskFunc = startFunc
					getSubtaskResultFunc = getFunc
				}

				// Registration
				if input.TaskName == "" {
					postTasksChan <- taskserver.PostRegisterTasksRequestObject{
						Body: &taskserver.PostRegisterTasksJSONRequestBody{
							Tasks: tasks,
						},
					}
				}

				return &taskserver.ServerHandler{
					Socket: socket,
					Input:  input,
					Channels: taskserver.ServerChannels{
						PostCallback: callbackChan,
						PostTasks:    postTasksChan,
					},
				}
			},
		},
		&noopStatusReporter{},
	)

	// Trigger a task and then we will simulate a subtask
	go func() {
		_, err = coordinator.StartTask(ctx, "test-task", []byte{}, nil)
		require.NoError(t, err)
	}()

	require.Eventually(t, func() bool {
		return startSubtaskFunc != nil
	}, time.Second*30, time.Millisecond*10)

	// Start a subtask
	go func() {
		taskRun, err := startSubtaskFunc("test-subtask", []byte("test-subtask-input"))
		require.NoError(t, err)
		subtaskRunID = taskRun.(taskserver.PostRunSubtask200JSONResponse).TaskRunId
	}()

	// Simulate a subtask callback
	subtaskPostCallbackChan <- taskserver.PostCallbackRequestObject{
		Body: &taskserver.CallbackRequest{
			Complete: &taskserver.TaskComplete{
				Output: []byte("subtask-done"),
			},
		},
	}

	// Wait for the subtask to complete
	require.Eventually(t, func() bool {
		taskRun, err := getSubtaskResultFunc(subtaskRunID)
		if err != nil {
			return false
		}

		return taskRun.(taskserver.PostGetSubtaskResult200JSONResponse).Complete != nil
	}, time.Second*2, time.Millisecond*10)
}

func TestPopulateTasksProcessExits(t *testing.T) {
	ctx := context.Background()

	s := store.NewTaskStore()

	postTasksChan := make(chan taskserver.PostRegisterTasksRequestObject)

	socketTracker, err := orchestrator.NewSocketTracker(ctx)
	require.NoError(t, err)

	coordinator := orchestrator.NewCoordinator(
		ctx,
		s,
		&fakeSdkExec{
			startService: func(ctx context.Context, taskRunID string, socket string, mode orchestrator.Mode) (func() error, <-chan error, error) {
				processDone := make(chan error, 1)
				processDone <- fmt.Errorf("exit status 1")
				return func() error { return nil }, processDone, nil
			},
		},
		socketTracker,
		&fakeServerFactory{
			newHandler: func(socket net.Listener, input taskserver.GetInput200JSONResponse, getSubtaskResultFunc taskserver.GetSubtaskResultFunc, startSubtaskFunc taskserver.StartSubtaskFunc) *taskserver.ServerHandler {
				return &taskserver.ServerHandler{
					Socket: socket,
					Input:  input,
					Channels: taskserver.ServerChannels{
						PostCallback: make(chan taskserver.PostCallbackRequestObject),
						PostTasks:    postTasksChan,
					},
				}
			},
		},
		&noopStatusReporter{},
	)

	_, err = coordinator.PopulateTasks(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "start command exited before registering tasks")
	require.Contains(t, err.Error(), "exit status 1")
	require.Contains(t, err.Error(), "--debug")
}

func TestStartTaskProcessExits(t *testing.T) {
	ctx := context.Background()

	s := store.NewTaskStore()

	tasks := []taskserver.Task{
		{
			Name: "test-task",
		},
	}

	postTasksChan := make(chan taskserver.PostRegisterTasksRequestObject, 1)
	postTasksChan <- taskserver.PostRegisterTasksRequestObject{
		Body: &taskserver.PostRegisterTasksJSONRequestBody{
			Tasks: tasks,
		},
	}

	socketTracker, err := orchestrator.NewSocketTracker(ctx)
	require.NoError(t, err)

	startCount := 0
	coordinator := orchestrator.NewCoordinator(
		ctx,
		s,
		&fakeSdkExec{
			startService: func(ctx context.Context, taskRunID string, socket string, mode orchestrator.Mode) (func() error, <-chan error, error) {
				startCount++
				if startCount == 1 {
					// Registration succeeds normally
					return func() error { return nil }, neverExits(), nil
				}
				// Task run: process exits immediately
				processDone := make(chan error, 1)
				processDone <- fmt.Errorf("exit status 1")
				return func() error { return nil }, processDone, nil
			},
		},
		socketTracker,
		&fakeServerFactory{
			newHandler: func(socket net.Listener, input taskserver.GetInput200JSONResponse, getSubtaskResultFunc taskserver.GetSubtaskResultFunc, startSubtaskFunc taskserver.StartSubtaskFunc) *taskserver.ServerHandler {
				return &taskserver.ServerHandler{
					Socket: socket,
					Input:  input,
					Channels: taskserver.ServerChannels{
						PostCallback: make(chan taskserver.PostCallbackRequestObject),
						PostTasks:    postTasksChan,
					},
				}
			},
		},
		&noopStatusReporter{},
	)

	taskRun, err := coordinator.StartTask(ctx, "test-task", []byte{}, nil)
	require.NoError(t, err)

	// The background goroutine should detect the process exit and fail the task run
	require.Eventually(t, func() bool {
		tr := s.GetTaskRun(taskRun.ID)
		return tr.Status == store.TaskRunStatusFailed
	}, time.Second*2, time.Millisecond*10)

	tr := s.GetTaskRun(taskRun.ID)
	require.Contains(t, *tr.Error, "start command exited before completing the task")
}

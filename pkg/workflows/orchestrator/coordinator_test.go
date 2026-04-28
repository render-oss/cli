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
func (noopStatusReporter) TaskCancelled(taskRun *store.TaskRun) {}
func (noopStatusReporter) TaskNotFound(taskSlug string)         {}

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
	harness := startTaskWithSubtask(t)

	// Simulate the subtask completing via a callback on its in-process
	// server.
	harness.subtaskPostCallback <- taskserver.PostCallbackRequestObject{
		Body: &taskserver.CallbackRequest{
			Complete: &taskserver.TaskComplete{
				Output: []byte("subtask-done"),
			},
		},
	}

	require.Eventually(t, func() bool {
		taskRun, err := harness.coordinator.GetSubtaskResult(harness.subtaskRunID)
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

func TestCancelTaskRun(t *testing.T) {
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
					return func() error { return nil }, neverExits(), nil
				}
				// Mirror the real exec behaviour: when ctx is canceled the
				// child process is killed and processDone fires with an
				// error.
				processDone := make(chan error, 1)
				go func() {
					<-ctx.Done()
					processDone <- fmt.Errorf("signal: killed")
				}()
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

	require.NoError(t, coordinator.CancelTaskRun(taskRun.ID))

	require.Eventually(t, func() bool {
		tr := s.GetTaskRun(taskRun.ID)
		return tr.Status != store.TaskRunStatusRunning
	}, time.Second*2, time.Millisecond*10)

	tr := s.GetTaskRun(taskRun.ID)
	require.Equal(t, store.TaskRunStatusCanceled, tr.Status,
		"expected canceled task run to have status %q, got %q",
		store.TaskRunStatusCanceled, tr.Status)
}

// cancellableSdkExec returns a fakeSdkExec that mirrors the real exec
// behavior: registration runs never exit on their own, and run-mode
// processes block until their context is canceled and then report a kill
// error on processDone.
func cancellableSdkExec() *fakeSdkExec {
	return &fakeSdkExec{
		startService: func(ctx context.Context, taskRunID string, socket string, mode orchestrator.Mode) (func() error, <-chan error, error) {
			if mode == orchestrator.ModeRegister {
				return func() error { return nil }, neverExits(), nil
			}
			processDone := make(chan error, 1)
			go func() {
				<-ctx.Done()
				processDone <- fmt.Errorf("signal: killed")
			}()
			return func() error { return nil }, processDone, nil
		},
	}
}

// parentSubtaskFactory builds a fakeServerFactory configured with the
// task list returned during registration, capturing the parent task's
// startSubtaskFunc into startSubtaskFuncOut so the test can spawn a
// subtask out-of-band.
func parentSubtaskFactory(
	tasks []taskserver.Task,
	postTasksChan chan taskserver.PostRegisterTasksRequestObject,
	subtaskPostCallbackChan chan taskserver.PostCallbackRequestObject,
	subtaskPostTasksChan chan taskserver.PostRegisterTasksRequestObject,
	startSubtaskFuncOut *taskserver.StartSubtaskFunc,
) *fakeServerFactory {
	return &fakeServerFactory{
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
				*startSubtaskFuncOut = startFunc
			}

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
					PostCallback: make(chan taskserver.PostCallbackRequestObject),
					PostTasks:    postTasksChan,
				},
			}
		},
	}
}

// taskWithSubtaskHarness is the shared fixture for the cancel and
// subtask-start tests: a coordinator with a started "test-task" root
// and an in-flight "test-subtask", both in TaskRunStatusRunning by the
// time it returns. subtaskPostCallback is the channel the subtask's
// in-process server reads from, so tests can simulate the subtask
// finishing by sending a CallbackRequest on it.
type taskWithSubtaskHarness struct {
	coordinator   *orchestrator.Coordinator
	store         *store.TaskStore
	parentTaskRun *store.TaskRun
	subtaskRunID  string

	subtaskPostCallback chan taskserver.PostCallbackRequestObject
}

// startTaskWithSubtask wires up the coordinator with cancellableSdkExec
// and parentSubtaskFactory, starts a "test-task" root, then drives a
// "test-subtask" through the captured startSubtaskFunc.
func startTaskWithSubtask(t *testing.T) *taskWithSubtaskHarness {
	t.Helper()

	ctx := context.Background()
	s := store.NewTaskStore()

	tasks := []taskserver.Task{
		{Name: "test-task"},
		{Name: "test-subtask"},
	}

	postTasksChan := make(chan taskserver.PostRegisterTasksRequestObject, 1)
	subtaskPostCallbackChan := make(chan taskserver.PostCallbackRequestObject)
	subtaskPostTasksChan := make(chan taskserver.PostRegisterTasksRequestObject, 1)

	socketTracker, err := orchestrator.NewSocketTracker(ctx)
	require.NoError(t, err)

	var startSubtaskFunc taskserver.StartSubtaskFunc

	coordinator := orchestrator.NewCoordinator(
		ctx,
		s,
		cancellableSdkExec(),
		socketTracker,
		parentSubtaskFactory(tasks, postTasksChan, subtaskPostCallbackChan, subtaskPostTasksChan, &startSubtaskFunc),
		&noopStatusReporter{},
	)

	parentTaskRun, err := coordinator.StartTask(ctx, "test-task", []byte{}, nil)
	require.NoError(t, err)

	// Wait for the parent task to start a subtask
	require.Eventually(t, func() bool {
		return startSubtaskFunc != nil
	}, time.Second*2, time.Millisecond*10)

	// Start the subtask & wait for it to start
	subtaskRunIDCh := make(chan string, 1)
	go func() {
		taskRun, err := startSubtaskFunc("test-subtask", []byte("test-subtask-input"))
		require.NoError(t, err)
		subtaskRunIDCh <- taskRun.(taskserver.PostRunSubtask200JSONResponse).TaskRunId
	}()

	var subtaskRunID string
	select {
	case subtaskRunID = <-subtaskRunIDCh:
	case <-time.After(time.Second * 2):
		t.Fatal("subtask did not start in time")
	}

	require.Equal(t, store.TaskRunStatusRunning, s.GetTaskRun(parentTaskRun.ID).Status)
	require.Equal(t, store.TaskRunStatusRunning, s.GetTaskRun(subtaskRunID).Status)

	return &taskWithSubtaskHarness{
		coordinator:         coordinator,
		store:               s,
		parentTaskRun:       parentTaskRun,
		subtaskRunID:        subtaskRunID,
		subtaskPostCallback: subtaskPostCallbackChan,
	}
}

func TestCancelTaskRunCascadesToSubtasks(t *testing.T) {
	harness := startTaskWithSubtask(t)

	require.NoError(t, harness.coordinator.CancelTaskRun(harness.parentTaskRun.ID))

	require.Eventually(t, func() bool {
		parent := harness.store.GetTaskRun(harness.parentTaskRun.ID)
		sub := harness.store.GetTaskRun(harness.subtaskRunID)
		return parent.Status == store.TaskRunStatusCanceled &&
			sub.Status == store.TaskRunStatusCanceled
	}, time.Second*2, time.Millisecond*10,
		"expected both root and subtask to be canceled; got root=%q sub=%q",
		harness.store.GetTaskRun(harness.parentTaskRun.ID).Status,
		harness.store.GetTaskRun(harness.subtaskRunID).Status)

	// The subtask records its lineage back to the root so callers can
	// observe that the cascade matched the actual parent/child link.
	sub := harness.store.GetTaskRun(harness.subtaskRunID)
	require.NotNil(t, sub.ParentTaskRunID)
	require.Equal(t, harness.parentTaskRun.ID, *sub.ParentTaskRunID)
	require.Equal(t, harness.parentTaskRun.ID, sub.RootTaskRunID)
}

func TestCancelTaskRunRejectsSubtask(t *testing.T) {
	harness := startTaskWithSubtask(t)

	// Directly canceling a subtask is not allowed: callers must cancel
	// the root and let the cascade reach it.
	err := harness.coordinator.CancelTaskRun(harness.subtaskRunID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "subtask")
	require.Contains(t, err.Error(), harness.parentTaskRun.ID,
		"error should point the caller at the root task run id")

	// Both task runs should remain running since the cancel was rejected.
	// Give any spurious cancellation goroutines a moment to (incorrectly)
	// run before asserting.
	require.Never(t, func() bool {
		parent := harness.store.GetTaskRun(harness.parentTaskRun.ID)
		sub := harness.store.GetTaskRun(harness.subtaskRunID)
		return parent.Status != store.TaskRunStatusRunning ||
			sub.Status != store.TaskRunStatusRunning
	}, time.Millisecond*200, time.Millisecond*20,
		"rejected subtask cancel should not transition any task run out of running")
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

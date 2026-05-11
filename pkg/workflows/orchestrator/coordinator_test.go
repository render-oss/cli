package orchestrator_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/pointers"
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

// TestSubtaskRetryKeepsParentAlive verifies that when a subtask fails but still
// has retries remaining, GetSubtaskResult reports StillRunning rather than
// surfacing the error to the parent. This prevents the parent from failing
// before the retry has a chance to run.
func TestSubtaskRetryKeepsParentAlive(t *testing.T) {
	ctx := context.Background()
	s := store.NewTaskStore()

	retryConfig := &taskserver.RetryConfig{
		MaxRetries:     pointers.From(2),
		WaitDurationMs: pointers.From[int64](100),
	}
	tasks := []taskserver.Task{
		{Name: "test-task"},
		{
			Name:    "test-subtask",
			Options: &taskserver.TaskOptions{Retry: retryConfig},
		},
	}

	postTasksChan := make(chan taskserver.PostRegisterTasksRequestObject, 1)
	subtaskPostCallbackChan := make(chan taskserver.PostCallbackRequestObject)
	subtaskPostTasksChan := make(chan taskserver.PostRegisterTasksRequestObject, 1)

	socketTracker, err := orchestrator.NewSocketTracker(ctx)
	require.NoError(t, err)

	var startSubtaskFunc taskserver.StartSubtaskFunc
	exec, runChan := controllableSdkExec()

	coordinator := orchestrator.NewCoordinator(
		ctx,
		s,
		exec,
		socketTracker,
		parentSubtaskFactory(tasks, postTasksChan, subtaskPostCallbackChan, subtaskPostTasksChan, &startSubtaskFunc),
		&noopStatusReporter{},
	)

	parentTaskRun, err := coordinator.StartTask(ctx, "test-task", []byte{}, nil)
	require.NoError(t, err)

	// Consume the parent's run handle so subsequent reads get the subtask's.
	<-runChan

	require.Eventually(t, func() bool {
		return startSubtaskFunc != nil
	}, time.Second*2, time.Millisecond*10)

	subtaskRunIDCh := make(chan string, 1)
	go func() {
		taskRun, err := startSubtaskFunc("test-subtask", []byte{})
		require.NoError(t, err)
		subtaskRunIDCh <- taskRun.(taskserver.PostRunSubtask200JSONResponse).TaskRunId
	}()

	var subtaskRunID string
	select {
	case subtaskRunID = <-subtaskRunIDCh:
	case <-time.After(time.Second * 2):
		t.Fatal("subtask did not start in time")
	}

	// Kill subtask attempt 1.
	subtaskHandle1 := <-runChan
	subtaskHandle1.done <- fmt.Errorf("exit status 1")

	// During the retry wait the subtask is Pending; GetSubtaskResult must
	// report StillRunning so the parent doesn't see the failure.
	require.Eventually(t, func() bool {
		return s.GetTaskRun(subtaskRunID).Status == store.TaskRunStatusPending
	}, time.Millisecond*200, time.Millisecond*5)

	result, err := coordinator.GetSubtaskResult(subtaskRunID)
	require.NoError(t, err)
	resp := result.(taskserver.PostGetSubtaskResult200JSONResponse)
	require.True(t, resp.StillRunning, "GetSubtaskResult should report still-running during retry wait")
	require.Nil(t, resp.Error, "GetSubtaskResult should not surface the failure during retry wait")

	// Parent must still be alive — it should not have been marked failed.
	require.Equal(t, store.TaskRunStatusRunning, s.GetTaskRun(parentTaskRun.ID).Status)

	// Subtask retry launches; let it succeed via callback.
	<-runChan // consume subtask attempt 2's handle

	subtaskPostCallbackChan <- taskserver.PostCallbackRequestObject{
		Body: &taskserver.CallbackRequest{
			Complete: &taskserver.TaskComplete{Output: json.RawMessage(`"ok"`)},
		},
	}

	// GetSubtaskResult should now return Complete.
	require.Eventually(t, func() bool {
		result, err := coordinator.GetSubtaskResult(subtaskRunID)
		if err != nil {
			return false
		}
		return result.(taskserver.PostGetSubtaskResult200JSONResponse).Complete != nil
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

	exec, _ := controllableSdkExec()
	coordinator := orchestrator.NewCoordinator(
		ctx,
		s,
		exec,
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

// runHandle exposes per-run controls for a fake exec process.
type runHandle struct {
	done chan error
}

// controllableSdkExec returns a fakeSdkExec where each run-mode start pushes
// a runHandle onto the returned channel so tests can drive process exits
// manually. It also mirrors real exec behavior by emitting "signal: killed"
// when ctx is canceled — whichever event fires first wins. Tests that don't
// care about manual exits can ignore the returned channel.
func controllableSdkExec() (*fakeSdkExec, chan runHandle) {
	runChan := make(chan runHandle, 16)
	exec := &fakeSdkExec{
		startService: func(ctx context.Context, taskRunID string, socket string, mode orchestrator.Mode) (func() error, <-chan error, error) {
			if mode == orchestrator.ModeRegister {
				return func() error { return nil }, neverExits(), nil
			}
			handle := runHandle{done: make(chan error, 1)}
			go func() {
				<-ctx.Done()
				select {
				case handle.done <- fmt.Errorf("signal: killed"):
				default:
				}
			}()
			runChan <- handle
			return func() error { return nil }, handle.done, nil
		},
	}
	return exec, runChan
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

	exec, _ := controllableSdkExec()
	coordinator := orchestrator.NewCoordinator(
		ctx,
		s,
		exec,
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

func TestRetryOnProcessExit(t *testing.T) {
	ctx := context.Background()
	s := store.NewTaskStore()

	// Non-zero WaitDurationMs gives a window where the run is reliably observable
	// in Pending status. With zero, the retry sleep returns immediately and the
	// Pending state exists for microseconds — too brief for the assertion to catch.
	//
	// With Factor=1.5 the per-retry sleep is waitMs * 1.5^attemptCount, so:
	//   retry 1 (attemptCount=0): 100ms
	//   retry 2 (attemptCount=1): 100ms * 1.5 = 150ms
	// We measure both elapsed waits below to verify exponential backoff is applied.
	retryConfig := &taskserver.RetryConfig{
		MaxRetries:     pointers.From(2),
		WaitDurationMs: pointers.From[int64](100),
		Factor:         pointers.From[float32](1.5),
	}
	tasks := []taskserver.Task{
		{
			Name: "test-task",
			Options: &taskserver.TaskOptions{
				Retry: retryConfig,
			},
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

	exec, runChan := controllableSdkExec()
	coordinator := orchestrator.NewCoordinator(
		ctx,
		s,
		exec,
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

	// Start the task run (attempt 1 of 3).
	taskRun, err := coordinator.StartTask(ctx, "test-task", []byte("input"), nil)
	require.NoError(t, err)
	require.Equal(t, store.TaskRunStatusRunning, s.GetTaskRun(taskRun.ID).Status)
	require.Empty(t, s.GetTaskRun(taskRun.ID).Attempts, "first attempt should have no prior attempts")

	// Kill attempt 1 → status should transition to pending during the wait,
	// then to running for retry 1.
	runHandle1 := <-runChan
	kill1At := time.Now()
	runHandle1.done <- fmt.Errorf("exit status 1")

	require.Eventually(t, func() bool {
		tr := s.GetTaskRun(taskRun.ID)
		return tr.Status == store.TaskRunStatusPending && len(tr.Attempts) == 1
	}, time.Millisecond*100, time.Millisecond*5, "task should be pending during retry wait")
	require.Equal(t, store.TaskRunStatusFailed, s.GetTaskRun(taskRun.ID).Attempts[0].Status)

	require.Eventually(t, func() bool {
		tr := s.GetTaskRun(taskRun.ID)
		return tr.Status == store.TaskRunStatusRunning && len(tr.Attempts) == 1
	}, time.Second*2, time.Millisecond*10, "retry 1 should start running after wait")
	retry1Wait := time.Since(kill1At)

	// Kill attempt 2 → pending → running for retry 2.
	runHandle2 := <-runChan
	kill2At := time.Now()
	runHandle2.done <- fmt.Errorf("exit status 1")

	require.Eventually(t, func() bool {
		tr := s.GetTaskRun(taskRun.ID)
		return tr.Status == store.TaskRunStatusPending && len(tr.Attempts) == 2
	}, time.Millisecond*100, time.Millisecond*5, "task should be pending during second retry wait")

	require.Eventually(t, func() bool {
		tr := s.GetTaskRun(taskRun.ID)
		return tr.Status == store.TaskRunStatusRunning && len(tr.Attempts) == 2
	}, time.Second*2, time.Millisecond*10, "retry 2 should start running after wait")
	retry2Wait := time.Since(kill2At)

	// Lower bounds confirm each wait actually happened. The differential check
	// below confirms Factor was applied (vs both waits being delayed by some
	// constant overhead).
	expectedRetry1Min := retryConfig.GetSleepDuration(0)
	expectedRetry2Min := retryConfig.GetSleepDuration(1)
	require.GreaterOrEqual(t, retry1Wait, expectedRetry1Min,
		"retry 1 should wait at least %s", expectedRetry1Min)
	require.GreaterOrEqual(t, retry2Wait, expectedRetry2Min,
		"retry 2 should wait at least %s", expectedRetry2Min)
	require.Greater(t, retry2Wait, retry1Wait+time.Millisecond*20,
		"retry 2 wait (%s) should be longer than retry 1 wait (%s) due to exponential backoff",
		retry2Wait, retry1Wait)

	// Kill attempt 3 — MaxRetries is 2, so no further retry should occur.
	runHandle3 := <-runChan
	runHandle3.done <- fmt.Errorf("exit status 1")

	require.Eventually(t, func() bool {
		return s.GetTaskRun(taskRun.ID).Status == store.TaskRunStatusFailed
	}, time.Second*2, time.Millisecond*10)

	// Still exactly one task run; no new runs created.
	require.Len(t, s.GetTaskRuns("test-task"), 1, "should not create new task runs on retry")
	require.Len(t, s.GetTaskRun(taskRun.ID).Attempts, 2, "two attempts archived, third is current")
}

func TestCancelTaskRunDuringRetryWait(t *testing.T) {
	ctx := context.Background()
	s := store.NewTaskStore()

	// Long enough wait that we can reliably observe and cancel during it.
	retryConfig := &taskserver.RetryConfig{
		MaxRetries:     pointers.From(2),
		WaitDurationMs: pointers.From[int64](500),
		Factor:         pointers.From[float32](1.0),
	}
	tasks := []taskserver.Task{
		{
			Name:    "test-task",
			Options: &taskserver.TaskOptions{Retry: retryConfig},
		},
	}

	postTasksChan := make(chan taskserver.PostRegisterTasksRequestObject, 1)
	postTasksChan <- taskserver.PostRegisterTasksRequestObject{
		Body: &taskserver.PostRegisterTasksJSONRequestBody{Tasks: tasks},
	}

	socketTracker, err := orchestrator.NewSocketTracker(ctx)
	require.NoError(t, err)

	exec, runChan := controllableSdkExec()
	coordinator := orchestrator.NewCoordinator(
		ctx, s, exec, socketTracker,
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

	taskRun, err := coordinator.StartTask(ctx, "test-task", []byte("input"), nil)
	require.NoError(t, err)

	// Kill attempt 1 → run should enter pending while it waits to retry.
	runHandle1 := <-runChan
	runHandle1.done <- fmt.Errorf("exit status 1")

	require.Eventually(t, func() bool {
		return s.GetTaskRun(taskRun.ID).Status == store.TaskRunStatusPending
	}, time.Millisecond*200, time.Millisecond*5,
		"task should be pending during retry wait")

	// While the run is in the retry-wait window it must still be cancelable.
	require.NoError(t, coordinator.CancelTaskRun(taskRun.ID),
		"canceling during retry wait should succeed")

	require.Eventually(t, func() bool {
		return s.GetTaskRun(taskRun.ID).Status == store.TaskRunStatusCanceled
	}, time.Second*1, time.Millisecond*5,
		"canceled run should reach canceled status")

	// No further retry attempt should be launched. Wait past the original
	// backoff to confirm the timer didn't fire a relaunch.
	require.Never(t, func() bool {
		select {
		case <-runChan:
			return true
		default:
			return false
		}
	}, time.Millisecond*800, time.Millisecond*50,
		"no retry should launch after cancellation")

	// Subsequent cancel attempts should report the run is no longer active.
	require.Error(t, coordinator.CancelTaskRun(taskRun.ID),
		"canceling an already-canceled run should error")
}

func TestNoRetryWithoutConfig(t *testing.T) {
	ctx := context.Background()
	s := store.NewTaskStore()

	tasks := []taskserver.Task{{Name: "test-task"}} // no RetryConfig

	postTasksChan := make(chan taskserver.PostRegisterTasksRequestObject, 1)
	postTasksChan <- taskserver.PostRegisterTasksRequestObject{
		Body: &taskserver.PostRegisterTasksJSONRequestBody{Tasks: tasks},
	}

	socketTracker, err := orchestrator.NewSocketTracker(ctx)
	require.NoError(t, err)

	processDone := make(chan error, 1)
	coordinator := orchestrator.NewCoordinator(
		ctx,
		s,
		&fakeSdkExec{
			startService: func(ctx context.Context, taskRunID string, socket string, mode orchestrator.Mode) (func() error, <-chan error, error) {
				if mode == orchestrator.ModeRegister {
					return func() error { return nil }, neverExits(), nil
				}
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

	processDone <- fmt.Errorf("exit status 1")

	require.Eventually(t, func() bool {
		return s.GetTaskRun(taskRun.ID).Status == store.TaskRunStatusFailed
	}, time.Second*2, time.Millisecond*10)

	require.Never(t, func() bool {
		tr := s.GetTaskRun(taskRun.ID)
		return tr.Status == store.TaskRunStatusRunning
	}, time.Millisecond*200, time.Millisecond*20, "no retry should occur without RetryConfig")
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

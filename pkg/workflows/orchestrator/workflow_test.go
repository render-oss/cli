//go:build !windows

package orchestrator_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/workflows/orchestrator"
	"github.com/render-oss/cli/pkg/workflows/store"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
	"github.com/stretchr/testify/require"
)

// sdkAttempt is one fake SDK process: a client on its socket, plus controls
// to crash it and to see whether the orchestrator killed it.
type sdkAttempt struct {
	client *taskserver.ClientWithResponses
	done   chan error
	killed <-chan struct{}
}

// workflowHarness runs a coordinator against the real socket server, with
// fake SDK processes that tests drive over the socket.
type workflowHarness struct {
	coordinator *orchestrator.Coordinator
	store       *store.TaskStore

	mu       sync.Mutex
	attempts map[string]chan *sdkAttempt
}

func newWorkflowHarness(t *testing.T, tasks []taskserver.Task) *workflowHarness {
	t.Helper()
	return newWorkflowHarnessWithReporter(t, tasks, &noopStatusReporter{})
}

func newWorkflowHarnessWithReporter(t *testing.T, tasks []taskserver.Task, reporter orchestrator.StatusReporter) *workflowHarness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	h := &workflowHarness{
		store:    store.NewTaskStore(),
		attempts: make(map[string]chan *sdkAttempt),
	}
	sockets, err := orchestrator.NewSocketTracker(ctx)
	require.NoError(t, err)
	factory := taskserver.NewTaskServerFactory()

	exec := &fakeSdkExec{
		startService: func(ctx context.Context, taskRunID string, socket string, mode orchestrator.Mode) (func() error, <-chan error, error) {
			if mode == orchestrator.ModeRegister {
				return func() error { return nil }, neverExits(), nil
			}
			attempt := &sdkAttempt{
				client: socketClient(t, socket),
				done:   make(chan error, 1),
				killed: ctx.Done(),
			}
			go func() {
				<-ctx.Done()
				select {
				case attempt.done <- fmt.Errorf("signal: killed"):
				default:
				}
			}()
			h.attemptChan(taskRunID) <- attempt
			return func() error { return nil }, attempt.done, nil
		},
	}

	h.coordinator = orchestrator.NewCoordinator(ctx, h.store, exec, sockets, &fakeServerFactory{
		newHandler: func(socket net.Listener, input taskserver.GetInput200JSONResponse, getResult taskserver.GetSubtaskResultFunc, startSubtask taskserver.StartSubtaskFunc) *taskserver.ServerHandler {
			handler := factory.NewHandler(socket, input, getResult, startSubtask, nil)
			if input.TaskName == "" {
				handler.Channels.PostTasks = make(chan taskserver.PostRegisterTasksRequestObject, 1)
				handler.Channels.PostTasks <- taskserver.PostRegisterTasksRequestObject{
					Body: &taskserver.Tasks{Tasks: tasks},
				}
			}
			return handler
		},
	}, reporter, orchestrator.WithRecoveryDelay(0))

	return h
}

func socketClient(t *testing.T, socket string) *taskserver.ClientWithResponses {
	httpClient := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		},
	}}
	client, err := taskserver.NewClientWithResponses("http://sdk", taskserver.WithHTTPClient(httpClient))
	require.NoError(t, err)
	return client
}

func (h *workflowHarness) attemptChan(taskRunID string) chan *sdkAttempt {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch, ok := h.attempts[taskRunID]
	if !ok {
		ch = make(chan *sdkAttempt, 256)
		h.attempts[taskRunID] = ch
	}
	return ch
}

// next waits for the next process started for a run.
func (h *workflowHarness) next(t *testing.T, taskRunID string) *sdkAttempt {
	t.Helper()
	select {
	case attempt := <-h.attemptChan(taskRunID):
		return attempt
	case <-time.After(5 * time.Second):
		t.Fatalf("no attempt started for %s", taskRunID)
		return nil
	}
}

// requireNoAttempt checks that no further process starts for a run.
func (h *workflowHarness) requireNoAttempt(t *testing.T, taskRunID string) {
	t.Helper()
	select {
	case <-h.attemptChan(taskRunID):
		t.Fatalf("unexpected attempt started for %s", taskRunID)
	case <-time.After(100 * time.Millisecond):
	}
}

func (h *workflowHarness) start(t *testing.T, taskName string) *store.TaskRun {
	t.Helper()
	run, err := h.coordinator.StartTask(context.Background(), taskName, []byte(`["o-1"]`), nil)
	require.NoError(t, err)
	return run
}

func (h *workflowHarness) requireStatus(t *testing.T, taskRunID string, status store.TaskRunStatus) {
	t.Helper()
	require.Eventually(t, func() bool {
		return h.store.GetTaskRun(taskRunID).Status == status
	}, 5*time.Second, 5*time.Millisecond, "run %s never reached %s", taskRunID, status)
}

func requireKilled(t *testing.T, attempt *sdkAttempt) {
	t.Helper()
	select {
	case <-attempt.killed:
	case <-time.After(5 * time.Second):
		t.Fatal("the attempt's process was not killed")
	}
}

func getInput(t *testing.T, attempt *sdkAttempt) *taskserver.InputResponse {
	t.Helper()
	// The SDK reports startup telemetry on /input; local dev ignores it.
	resp, err := attempt.client.GetInputWithResponse(context.Background(), &taskserver.GetInputParams{
		SdkStartedAt: pointers.From(fmt.Sprint(time.Now().UnixNano())),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.JSON200, "status %d", resp.StatusCode())
	return resp.JSON200
}

func key(seq int) taskserver.StepKey {
	return taskserver.StepKey{Seq: seq}
}

func startStep(t *testing.T, attempt *sdkAttempt, seq int, input []byte) *taskserver.PostStepInlineStartResponse {
	t.Helper()
	resp, err := attempt.client.PostStepInlineStartWithResponse(context.Background(), taskserver.StepStartRequest{
		StepKey: key(seq), TaskName: "charge", Input: &input,
	})
	require.NoError(t, err)
	return resp
}

func failAttempt(t *testing.T, attempt *sdkAttempt, seq, n int, message string) *taskserver.PostStepInlineAttemptResponse {
	t.Helper()
	resp, err := attempt.client.PostStepInlineAttemptWithResponse(context.Background(), taskserver.StepAttemptRequest{
		StepKey: key(seq), Attempt: n, Error: taskserver.StepError{Name: "Error", Message: message},
	})
	require.NoError(t, err)
	return resp
}

func postComplete(t *testing.T, attempt *sdkAttempt, req taskserver.StepCompleteRequest) *taskserver.PostStepInlineCompleteResponse {
	t.Helper()
	resp, err := attempt.client.PostStepInlineCompleteWithResponse(context.Background(), req)
	require.NoError(t, err)
	return resp
}

func jsonPayload(data []byte) *taskserver.Payload {
	return &taskserver.Payload{Serializer: 1, Data: &data}
}

func completeStep(t *testing.T, attempt *sdkAttempt, runID string, seq int, output []byte) *taskserver.PostStepInlineCompleteResponse {
	t.Helper()
	return postComplete(t, attempt, taskserver.StepCompleteRequest{
		SessionId: runID, Key: key(seq),
		Outcome: taskserver.StepOutcome{CompletedAt: time.Now(), Output: jsonPayload(output)},
	})
}

// failedCompletion completes seq with a final attempt that failed with message.
func failedCompletion(runID string, seq, n int, message string) taskserver.StepCompleteRequest {
	now := time.Now()
	stepErr := fmt.Sprintf(`{"name":"Error","message":%q}`, message)
	return taskserver.StepCompleteRequest{
		SessionId: runID, Key: key(seq),
		Outcome: taskserver.StepOutcome{
			CompletedAt: now,
			FinalAttempt: &taskserver.StepAttempt{
				SessionId: runID, Key: key(seq), Attempt: n,
				StartedAt: now.Add(-time.Second), EndedAt: now,
				Error: *jsonPayload([]byte(stepErr)),
			},
		},
	}
}

func callback(t *testing.T, attempt *sdkAttempt, body taskserver.CallbackRequest) {
	t.Helper()
	resp, err := attempt.client.PostCallbackWithResponse(context.Background(), body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
}

var workflowTasks = []taskserver.Task{
	{Name: "wf", Workflow: pointers.From(true)},
	{Name: "invoice"},
}

func TestWorkflowCrashRecoveryReplaysHistory(t *testing.T) {
	h := newWorkflowHarness(t, workflowTasks)
	run := h.start(t, "wf")

	first := h.next(t, run.ID)
	input := getInput(t, first)
	require.True(t, *input.IsWorkflow)
	require.Equal(t, 0, *input.Attempt)
	require.Equal(t, taskserver.History{}, *input.History, "attempt 0 has no history")

	start := startStep(t, first, 0, []byte(`["a"]`))
	require.True(t, start.JSON200.Applied)
	require.Nil(t, start.JSON200.Step, "an applied start returns no step")
	complete := completeStep(t, first, run.ID, 0, []byte(`["ok"]`))
	require.True(t, complete.JSON200.Applied)
	require.Nil(t, complete.JSON200.Outcome, "an applied complete returns no outcome")
	startStep(t, first, 1, []byte(`[]`))
	require.True(t, failAttempt(t, first, 1, 0, "card declined").JSON200.Applied)
	require.False(t, failAttempt(t, first, 1, 0, "card declined").JSON200.Applied, "a repeated attempt isn't recorded twice")

	first.done <- fmt.Errorf("exit status 1")

	second := h.next(t, run.ID)
	input = getInput(t, second)
	require.Equal(t, 1, *input.Attempt)
	steps := *input.History.Steps
	require.Len(t, steps, 2)
	require.NotNil(t, steps[0].CompletedAt)
	require.Equal(t, []byte(`["ok"]`), *steps[0].Output)
	require.Equal(t, 0, steps[0].WorkflowAttempt)
	require.Nil(t, steps[1].CompletedAt)
	require.Len(t, *input.History.StepAttempts, 1)
	require.Equal(t, 1, h.store.GetTaskRun(run.ID).Recoveries)

	// A replayed start loses and gets the recorded row back.
	replayed := startStep(t, second, 0, []byte(`["a"]`))
	require.False(t, replayed.JSON200.Applied)
	require.NotNil(t, replayed.JSON200.Step.CompletedAt)

	// A replayed complete loses and gets the recorded outcome back.
	recompleted := completeStep(t, second, run.ID, 0, []byte(`["different"]`))
	require.False(t, recompleted.JSON200.Applied)
	require.Equal(t, []byte(`["ok"]`), *recompleted.JSON200.Outcome.Output)

	// workflow_attempt is the attempt that completed the step.
	require.True(t, completeStep(t, second, run.ID, 1, []byte(`["paid"]`)).JSON200.Applied)
	step, _ := h.store.GetStep(run.ID, 1)
	require.Equal(t, 1, step.WorkflowAttempt)

	callback(t, second, taskserver.CallbackRequest{Complete: &taskserver.TaskComplete{Output: []byte(`["done"]`)}})
	h.requireStatus(t, run.ID, store.TaskRunStatusComplete)
}

// stepEventReporter records the step events a run reports.
type stepEventReporter struct {
	noopStatusReporter
	mu     sync.Mutex
	events []string
}

func (r *stepEventReporter) record(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *stepEventReporter) StepStarted(run *store.TaskRun, step taskserver.StepRecord) {
	r.record(fmt.Sprintf("%s started %d", run.ID, step.StepKey.Seq))
}

func (r *stepEventReporter) StepRetrying(run *store.TaskRun, step taskserver.StepRecord, attempt taskserver.StepAttemptRecord) {
	r.record(fmt.Sprintf("%s retrying %d attempt %d: %s", run.ID, step.StepKey.Seq, attempt.Attempt, attempt.Error.Message))
}

func (r *stepEventReporter) StepCompleted(run *store.TaskRun, step taskserver.StepRecord) {
	if step.Error != nil {
		r.record(fmt.Sprintf("%s failed %d: %s", run.ID, step.StepKey.Seq, step.Error.Message))
		return
	}
	r.record(fmt.Sprintf("%s completed %d: %s", run.ID, step.StepKey.Seq, *step.Output))
}

func (r *stepEventReporter) Events() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

// Only applied step writes are reported, so a replayed attempt doesn't log
// its steps again.
func TestWorkflowReportsSteps(t *testing.T) {
	reporter := &stepEventReporter{}
	h := newWorkflowHarnessWithReporter(t, workflowTasks, reporter)
	run := h.start(t, "wf")

	first := h.next(t, run.ID)
	getInput(t, first)
	startStep(t, first, 0, []byte(`["a"]`))
	completeStep(t, first, run.ID, 0, []byte(`["ok"]`))
	startStep(t, first, 1, []byte(`[]`))
	failAttempt(t, first, 1, 0, "timeout")
	failAttempt(t, first, 1, 0, "timeout")
	postComplete(t, first, failedCompletion(run.ID, 1, 1, "card declined"))

	first.done <- fmt.Errorf("exit status 1")
	second := h.next(t, run.ID)
	getInput(t, second)
	startStep(t, second, 0, []byte(`["a"]`))
	completeStep(t, second, run.ID, 0, []byte(`["ok"]`))

	require.Equal(t, []string{
		run.ID + " started 0",
		run.ID + ` completed 0: ["ok"]`,
		run.ID + " started 1",
		run.ID + " retrying 1 attempt 0: timeout",
		run.ID + " failed 1: card declined",
	}, reporter.Events())
}

func TestWorkflowStepInlineMissingStart(t *testing.T) {
	h := newWorkflowHarness(t, workflowTasks)
	run := h.start(t, "wf")
	attempt := h.next(t, run.ID)

	complete := completeStep(t, attempt, run.ID, 0, []byte(`[1]`))
	require.NotNil(t, complete.JSON409)
	require.Equal(t, taskserver.MissingStart, complete.JSON409.Code)

	failed := failAttempt(t, attempt, 0, 0, "boom")
	require.NotNil(t, failed.JSON409)
	require.Equal(t, taskserver.MissingStart, failed.JSON409.Code)
}

func TestWorkflowStepInlineFinalAttempt(t *testing.T) {
	h := newWorkflowHarness(t, workflowTasks)
	run := h.start(t, "wf")
	first := h.next(t, run.ID)

	startStep(t, first, 0, []byte(`[]`))
	require.True(t, failAttempt(t, first, 0, 0, "timeout").JSON200.Applied)
	complete := postComplete(t, first, failedCompletion(run.ID, 0, 1, "card declined"))
	require.True(t, complete.JSON200.Applied)

	step, _ := h.store.GetStep(run.ID, 0)
	require.NotNil(t, step.CompletedAt)
	require.Equal(t, "card declined", step.Error.Message)
	require.Nil(t, step.Output)
	require.False(t, failAttempt(t, first, 0, 1, "late").JSON200.Applied, "the final attempt is recorded with the outcome")

	first.done <- fmt.Errorf("exit status 1")
	second := h.next(t, run.ID)
	getInput(t, second)

	// A replayed complete gets the recorded error back.
	recompleted := completeStep(t, second, run.ID, 0, []byte(`["ok"]`))
	require.False(t, recompleted.JSON200.Applied)
	require.Equal(t, "card declined", recompleted.JSON200.Outcome.Error.Message)
}

func TestWorkflowStepInlineCompleteValidation(t *testing.T) {
	tests := []struct {
		name   string
		modify func(req *taskserver.StepCompleteRequest)
	}{
		{
			name:   "session_id of another run",
			modify: func(req *taskserver.StepCompleteRequest) { req.SessionId = "trn-other" },
		},
		{
			name:   "missing completed_at",
			modify: func(req *taskserver.StepCompleteRequest) { req.Outcome.CompletedAt = time.Time{} },
		},
		{
			name: "both output and final_attempt",
			modify: func(req *taskserver.StepCompleteRequest) {
				req.Outcome.Output = jsonPayload([]byte(`1`))
			},
		},
		{
			name: "final_attempt for another step",
			modify: func(req *taskserver.StepCompleteRequest) {
				req.Outcome.FinalAttempt.Key = key(1)
			},
		},
		{
			name: "final_attempt ended_at differs from completed_at",
			modify: func(req *taskserver.StepCompleteRequest) {
				req.Outcome.FinalAttempt.EndedAt = req.Outcome.CompletedAt.Add(-time.Millisecond)
			},
		},
		{
			name: "error without a name",
			modify: func(req *taskserver.StepCompleteRequest) {
				req.Outcome.FinalAttempt.Error = *jsonPayload([]byte(`{"message":"boom"}`))
			},
		},
		{
			name: "payload with a url",
			modify: func(req *taskserver.StepCompleteRequest) {
				req.Outcome.FinalAttempt = nil
				req.Outcome.Output = &taskserver.Payload{Serializer: 1, Url: pointers.From("s3://output")}
			},
		},
		{
			name: "non-JSON serializer",
			modify: func(req *taskserver.StepCompleteRequest) {
				req.Outcome.FinalAttempt = nil
				req.Outcome.Output = &taskserver.Payload{Serializer: 2, Data: pointers.From([]byte(`1`))}
			},
		},
		{
			name: "completed before the step started",
			modify: func(req *taskserver.StepCompleteRequest) {
				past := time.Now().Add(-time.Hour)
				req.Outcome.CompletedAt = past
				req.Outcome.FinalAttempt.StartedAt = past.Add(-time.Second)
				req.Outcome.FinalAttempt.EndedAt = past
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newWorkflowHarness(t, workflowTasks)
			run := h.start(t, "wf")
			attempt := h.next(t, run.ID)
			startStep(t, attempt, 0, []byte(`[]`))

			req := failedCompletion(run.ID, 0, 0, "boom")
			tt.modify(&req)
			resp := postComplete(t, attempt, req)

			require.Equal(t, http.StatusBadRequest, resp.StatusCode())
			step, _ := h.store.GetStep(run.ID, 0)
			require.Nil(t, step.CompletedAt)
		})
	}
}

func TestWorkflowInvokedTask(t *testing.T) {
	h := newWorkflowHarness(t, workflowTasks)
	run := h.start(t, "wf")
	first := h.next(t, run.ID)

	runSubtask := func(attempt *sdkAttempt) string {
		resp, err := attempt.client.PostRunSubtaskWithResponse(context.Background(), taskserver.RunSubtaskRequest{
			TaskName: "invoice", Input: pointers.From([]byte(`["o-1"]`)),
			StepKey: pointers.From(key(0)),
		})
		require.NoError(t, err)
		require.NotNil(t, resp.JSON200, "status %d", resp.StatusCode())
		return resp.JSON200.TaskRunId
	}
	stepWait := func(attempt *sdkAttempt, childID string) *taskserver.PostStepWaitResponse {
		resp, err := attempt.client.PostStepWaitWithResponse(context.Background(), taskserver.WaitRequest{Seq: 0, ChildRunId: childID})
		require.NoError(t, err)
		return resp
	}

	childID := runSubtask(first)
	require.Equal(t, childID, runSubtask(first), "a retry for the same seq gets the same child")
	child := h.next(t, childID)

	wrongChild := stepWait(first, "trn-other")
	require.Equal(t, http.StatusBadRequest, wrongChild.StatusCode())

	wait := stepWait(first, childID)
	require.Equal(t, taskserver.WaitResponseActionSuspending, wait.JSON200.Action)
	requireKilled(t, first)
	h.requireStatus(t, run.ID, store.TaskRunStatusPaused)
	h.requireNoAttempt(t, run.ID)

	callback(t, child, taskserver.CallbackRequest{Complete: &taskserver.TaskComplete{Output: []byte(`["inv-1"]`)}})

	second := h.next(t, run.ID)
	input := getInput(t, second)
	require.Equal(t, 1, *input.Attempt)
	require.Equal(t, childID, *(*input.History.Steps)[0].ChildRunId)

	wait = stepWait(second, childID)
	require.Equal(t, taskserver.WaitResponseActionDone, wait.JSON200.Action)
	require.Equal(t, []byte(`["inv-1"]`), wait.JSON200.Complete.Output)
	step, _ := h.store.GetStep(run.ID, 0)
	require.NotNil(t, step.CompletedAt)
	require.Equal(t, []byte(`["inv-1"]`), *step.Output)
}

func TestWorkflowDeterminismErrorEndsRun(t *testing.T) {
	h := newWorkflowHarness(t, workflowTasks)
	run := h.start(t, "wf")
	attempt := h.next(t, run.ID)

	callback(t, attempt, taskserver.CallbackRequest{Error: &taskserver.TaskError{
		Details: "determinism",
		Reason:  pointers.From(taskserver.TaskErrorReasonDeterminismError),
		Determinism: &taskserver.DeterminismDetails{
			StepKey: key(0), Kind: taskserver.InputHash,
			Expected: pointers.From(`inline lookup("a")`), Actual: pointers.From(`inline lookup("b")`),
		},
	}})

	h.requireStatus(t, run.ID, store.TaskRunStatusFailed)
	require.Equal(t,
		`determinism error at step 0 (kind input_hash): expected inline lookup("a"), got inline lookup("b")`,
		*h.store.GetTaskRun(run.ID).Error)
	h.requireNoAttempt(t, run.ID)
}

func TestWorkflowLimitErrorEndsRun(t *testing.T) {
	h := newWorkflowHarness(t, workflowTasks)
	run := h.start(t, "wf")
	attempt := h.next(t, run.ID)

	callback(t, attempt, taskserver.CallbackRequest{Error: &taskserver.TaskError{
		Details: "the step's output is too large",
		Reason:  pointers.From(taskserver.TaskErrorReasonWorkflowPayloadTooLarge),
		Limit:   pointers.From(int64(1024)),
		Value:   pointers.From(int64(2048)),
	}})

	h.requireStatus(t, run.ID, store.TaskRunStatusFailed)
	require.Equal(t,
		`workflow_payload_too_large (2048, limit 1024): the step's output is too large`,
		*h.store.GetTaskRun(run.ID).Error)
}

func TestWorkflowSystemErrorReplays(t *testing.T) {
	h := newWorkflowHarness(t, workflowTasks)
	run := h.start(t, "wf")

	first := h.next(t, run.ID)
	startStep(t, first, 0, []byte(`["a"]`))
	completeStep(t, first, run.ID, 0, []byte(`["ok"]`))
	callback(t, first, taskserver.CallbackRequest{Error: &taskserver.TaskError{
		Details: "couldn't reach the object store",
		Reason:  pointers.From(taskserver.TaskErrorReasonSystemError),
	}})

	second := h.next(t, run.ID)
	input := getInput(t, second)
	require.Equal(t, 1, *input.Attempt)
	require.Len(t, *input.History.Steps, 1, "the replay gets the recorded step")
	require.Equal(t, 1, h.store.GetTaskRun(run.ID).Recoveries)

	callback(t, second, taskserver.CallbackRequest{Complete: &taskserver.TaskComplete{Output: []byte(`["done"]`)}})
	h.requireStatus(t, run.ID, store.TaskRunStatusComplete)
}

func TestWorkflowSystemErrorRecoveryBudget(t *testing.T) {
	h := newWorkflowHarness(t, workflowTasks)
	run := h.start(t, "wf")

	for range 101 {
		callback(t, h.next(t, run.ID), taskserver.CallbackRequest{Error: &taskserver.TaskError{
			Details: "couldn't reach the object store",
			Reason:  pointers.From(taskserver.TaskErrorReasonSystemError),
		}})
	}

	h.requireStatus(t, run.ID, store.TaskRunStatusFailed)
	require.Equal(t,
		"workflow attempt exited before completing; recovery budget of 100 exhausted: system error: couldn't reach the object store",
		*h.store.GetTaskRun(run.ID).Error)
	h.requireNoAttempt(t, run.ID)
}

func TestWorkflowRecoveryBudget(t *testing.T) {
	h := newWorkflowHarness(t, workflowTasks)
	run := h.start(t, "wf")

	for range 101 {
		h.next(t, run.ID).done <- fmt.Errorf("exit status 1")
	}

	h.requireStatus(t, run.ID, store.TaskRunStatusFailed)
	require.Contains(t, *h.store.GetTaskRun(run.ID).Error, "recovery budget of 100 exhausted")
	h.requireNoAttempt(t, run.ID)
}

func TestWorkflowCancel(t *testing.T) {
	t.Run("step routes answer run_cancelled", func(t *testing.T) {
		h := newWorkflowHarness(t, workflowTasks)
		run := h.start(t, "wf")
		attempt := h.next(t, run.ID)

		require.NoError(t, h.coordinator.CancelTaskRun(run.ID))
		requireKilled(t, attempt)

		resp := startStep(t, attempt, 0, []byte(`[]`))
		require.NotNil(t, resp.JSON409)
		require.Equal(t, taskserver.RunCancelled, resp.JSON409.Code)
	})

	t.Run("a paused run and its child are canceled", func(t *testing.T) {
		h := newWorkflowHarness(t, workflowTasks)
		run := h.start(t, "wf")
		attempt := h.next(t, run.ID)

		sub, err := attempt.client.PostRunSubtaskWithResponse(context.Background(), taskserver.RunSubtaskRequest{
			TaskName: "invoice", StepKey: pointers.From(key(0)),
		})
		require.NoError(t, err)
		childID := sub.JSON200.TaskRunId
		child := h.next(t, childID)
		_, err = attempt.client.PostStepWaitWithResponse(context.Background(), taskserver.WaitRequest{Seq: 0, ChildRunId: childID})
		require.NoError(t, err)
		h.requireStatus(t, run.ID, store.TaskRunStatusPaused)

		require.NoError(t, h.coordinator.CancelTaskRun(run.ID))

		h.requireStatus(t, run.ID, store.TaskRunStatusCanceled)
		h.requireStatus(t, childID, store.TaskRunStatusCanceled)
		requireKilled(t, child)
		h.requireNoAttempt(t, run.ID)
	})
}

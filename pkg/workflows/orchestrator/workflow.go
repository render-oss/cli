package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/workflows/store"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
)

const (
	// maxRecoveries is how many crashed attempts a workflow run may replay.
	// Wakes after a child finishes don't count. It's a local-only guard so a
	// crash-looping SDK eventually fails the run instead of spinning; the
	// platform replays without limit.
	maxRecoveries = 100
	// defaultRecoveryDelay keeps a crash-looping SDK from spinning.
	defaultRecoveryDelay = 500 * time.Millisecond
)

// suspendSignal tells watchAttempt that the attempt answered "suspending":
// the attempt ends and the run pauses until childRunID finishes.
type suspendSignal struct {
	childRunID string
}

// workflowAttempt serves the step routes for one attempt of a workflow run.
type workflowAttempt struct {
	coordinator *Coordinator
	runID       string
	attempt     int
	suspend     chan suspendSignal
}

func newWorkflowAttempt(c *Coordinator, taskRun *store.TaskRun) *workflowAttempt {
	return &workflowAttempt{
		coordinator: c,
		runID:       taskRun.ID,
		attempt:     taskRun.WorkflowAttempt,
		suspend:     make(chan suspendSignal, 1),
	}
}

func (a *workflowAttempt) cancelled() bool {
	taskRun := a.coordinator.store.GetTaskRun(a.runID)
	return taskRun == nil || taskRun.Status == store.TaskRunStatusCanceled
}

func (a *workflowAttempt) signalSuspend(sig suspendSignal) {
	select {
	case a.suspend <- sig:
	default:
	}
}

func runCancelled() taskserver.StepConflictError {
	return taskserver.StepConflictError{Code: taskserver.RunCancelled, Message: "workflow run was canceled"}
}

func (a *workflowAttempt) StepInlineStart(_ context.Context, req taskserver.StepStartRequest) (taskserver.PostStepInlineStartResponseObject, error) {
	if a.cancelled() {
		return taskserver.PostStepInlineStart409JSONResponse(runCancelled()), nil
	}

	step, applied := a.coordinator.store.StartStep(a.runID, taskserver.StepRecord{
		StepKey:   req.StepKey,
		Type:      store.StepTypeInline,
		TaskName:  pointers.From(req.TaskName),
		Input:     req.Input,
		StartedAt: pointers.ValueOrDefault(req.StartedAt, a.coordinator.store.Now()),
	})
	resp := taskserver.PostStepInlineStart200JSONResponse{Applied: applied}
	if !applied {
		resp.Step = &step
	}
	return resp, nil
}

func (a *workflowAttempt) StepInlineAttempt(_ context.Context, req taskserver.StepAttemptRequest) (taskserver.PostStepInlineAttemptResponseObject, error) {
	if a.cancelled() {
		return taskserver.PostStepInlineAttempt409JSONResponse(runCancelled()), nil
	}
	if _, ok := a.coordinator.store.GetStep(a.runID, req.StepKey.Seq); !ok {
		return taskserver.PostStepInlineAttempt409JSONResponse(missingStart(req.StepKey.Seq)), nil
	}

	_, applied := a.coordinator.store.RecordStepAttempt(a.runID, taskserver.StepAttemptRecord{
		StepKey:         req.StepKey,
		Attempt:         req.Attempt,
		WorkflowAttempt: a.attempt,
		Error:           req.Error,
		StartedAt:       req.StartedAt,
	})
	return taskserver.PostStepInlineAttempt200JSONResponse{Applied: applied}, nil
}

func (a *workflowAttempt) StepInlineComplete(_ context.Context, req taskserver.StepCompleteRequest) (taskserver.PostStepInlineCompleteResponseObject, error) {
	if a.cancelled() {
		return taskserver.PostStepInlineComplete409JSONResponse(runCancelled()), nil
	}
	completion, err := a.stepCompletion(req)
	if err != nil {
		return taskserver.PostStepInlineComplete400Response{}, nil
	}

	step, applied, err := a.coordinator.store.CompleteStep(a.runID, req.Key.Seq, completion)
	switch {
	case errors.Is(err, store.ErrMissingStart):
		return taskserver.PostStepInlineComplete409JSONResponse(missingStart(req.Key.Seq)), nil
	case errors.Is(err, store.ErrCompletedBeforeStart):
		return taskserver.PostStepInlineComplete400Response{}, nil
	case err != nil:
		return nil, err
	}
	resp := taskserver.PostStepInlineComplete200JSONResponse{Applied: applied}
	if !applied {
		resp.Outcome = &taskserver.StepResult{Output: step.Output, Error: step.Error}
	}
	return resp, nil
}

// stepCompletion validates a /step-inline/complete request the way renderd
// does, and converts it to the store's completion.
func (a *workflowAttempt) stepCompletion(req taskserver.StepCompleteRequest) (store.StepCompletion, error) {
	outcome := req.Outcome
	if req.SessionId != a.runID {
		return store.StepCompletion{}, errors.New("session_id must match the workflow run")
	}
	if outcome.CompletedAt.IsZero() {
		return store.StepCompletion{}, errors.New("completed_at is required")
	}
	if outcome.Output != nil && outcome.FinalAttempt != nil {
		return store.StepCompletion{}, errors.New("an outcome has an output or a final_attempt, not both")
	}

	output, err := payloadData(outcome.Output)
	if err != nil {
		return store.StepCompletion{}, err
	}
	completion := store.StepCompletion{
		WorkflowAttempt: a.attempt,
		CompletedAt:     outcome.CompletedAt,
		Output:          output,
	}

	final := outcome.FinalAttempt
	if final == nil {
		return completion, nil
	}
	if final.SessionId != req.SessionId || !sameStepKey(final.Key, req.Key) {
		return store.StepCompletion{}, errors.New("final_attempt session_id and key must match the completion")
	}
	if final.Attempt < 0 {
		return store.StepCompletion{}, fmt.Errorf("attempt %d is out of range", final.Attempt)
	}
	if final.StartedAt.IsZero() || final.StartedAt.After(final.EndedAt) {
		return store.StepCompletion{}, errors.New("invalid attempt start or end time")
	}
	if !final.EndedAt.Equal(outcome.CompletedAt) {
		return store.StepCompletion{}, errors.New("final_attempt ended_at must match completed_at")
	}
	stepErr, err := stepErrorFromPayload(final.Error)
	if err != nil {
		return store.StepCompletion{}, err
	}
	completion.Error = stepErr
	completion.FinalAttempt = &taskserver.StepAttemptRecord{
		StepKey:         final.Key,
		Attempt:         final.Attempt,
		WorkflowAttempt: a.attempt,
		Error:           *stepErr,
		StartedAt:       pointers.From(final.StartedAt),
		EndedAt:         final.EndedAt,
	}
	return completion, nil
}

// jsonSerializer is the Payload serializer for JSON, the only one the SDK sends.
const jsonSerializer = 1

// payloadData returns an inline JSON payload's bytes. Storage URLs are
// server-owned, so the SDK never sends one.
func payloadData(p *taskserver.Payload) (*[]byte, error) {
	if p == nil {
		return nil, nil
	}
	if p.Url != nil {
		return nil, errors.New("payload url must not be supplied")
	}
	if p.Serializer != jsonSerializer {
		return nil, fmt.Errorf("unsupported payload serializer %d", p.Serializer)
	}
	var data []byte
	if p.Data != nil {
		data = *p.Data
	}
	return &data, nil
}

// stepErrorFromPayload decodes a final attempt's error, which must be a
// StepError with a name.
func stepErrorFromPayload(p taskserver.Payload) (*taskserver.StepError, error) {
	data, err := payloadData(&p)
	if err != nil {
		return nil, err
	}
	if len(*data) == 0 {
		return nil, errors.New("error requires inline data")
	}
	var stepErr taskserver.StepError
	if err := json.Unmarshal(*data, &stepErr); err != nil {
		return nil, fmt.Errorf("invalid error: %w", err)
	}
	if stepErr.Name == "" {
		return nil, errors.New("error.name is required")
	}
	return &stepErr, nil
}

func sameStepKey(a, b taskserver.StepKey) bool {
	return a.Seq == b.Seq && pointers.StringValue(a.SubKey) == pointers.StringValue(b.SubKey)
}

func missingStart(seq int) taskserver.StepConflictError {
	return taskserver.StepConflictError{
		Code:    taskserver.MissingStart,
		Message: fmt.Sprintf("step %d has no start row", seq),
	}
}

func (a *workflowAttempt) StepWait(_ context.Context, req taskserver.WaitRequest) (taskserver.PostStepWaitResponseObject, error) {
	if a.cancelled() {
		return taskserver.PostStepWait409JSONResponse(runCancelled()), nil
	}

	step, ok := a.coordinator.store.GetStep(a.runID, req.Seq)
	if !ok || step.Type != store.StepTypeInvoke || pointers.StringValue(step.ChildRunId) != req.ChildRunId {
		return taskserver.PostStepWait400Response{}, nil
	}

	if step.CompletedAt == nil {
		child := a.coordinator.store.GetTaskRun(req.ChildRunId)
		if child == nil {
			return taskserver.PostStepWait400Response{}, nil
		}
		complete, taskErr, done := a.coordinator.subtaskOutcome(child)
		if !done {
			a.signalSuspend(suspendSignal{childRunID: req.ChildRunId})
			return taskserver.PostStepWait200JSONResponse{Action: taskserver.WaitResponseActionSuspending}, nil
		}
		// Complete the row before answering, so a replay sees the outcome.
		step, _ = a.completeInvokeStep(req.Seq, complete, taskErr)
	}

	return waitDone(step), nil
}

func (a *workflowAttempt) completeInvokeStep(seq int, complete *taskserver.TaskComplete, taskErr *taskserver.TaskError) (taskserver.StepRecord, error) {
	completion := store.StepCompletion{WorkflowAttempt: a.attempt}
	if taskErr != nil {
		completion.Error = &taskserver.StepError{Name: "TaskError", Message: taskErr.Details, Stack: taskErr.StackTrace}
	} else if complete != nil {
		completion.Output = pointers.From(complete.Output)
	}
	step, _, err := a.coordinator.store.CompleteStep(a.runID, seq, completion)
	return step, err
}

// waitDone answers /step-wait from a completed invoke step row.
func waitDone(step taskserver.StepRecord) taskserver.PostStepWait200JSONResponse {
	resp := taskserver.PostStepWait200JSONResponse{Action: taskserver.WaitResponseActionDone}
	if step.Error != nil {
		resp.Error = &taskserver.TaskError{Details: step.Error.Message, StackTrace: step.Error.Stack}
		return resp
	}
	resp.Complete = &taskserver.TaskComplete{Output: pointers.ValueOrDefault(step.Output, []byte{})}
	return resp
}

// RunSubtask dispatches an invoked task and records its start row. It backs
// ctx.invoke in the SDK. A retry
// for the same seq gets the child that was already started.
func (a *workflowAttempt) RunSubtask(req taskserver.RunSubtaskRequest) (taskserver.PostRunSubtaskResponseObject, error) {
	parent := a.coordinator.store.GetTaskRun(a.runID)
	if parent == nil || parent.Status == store.TaskRunStatusCanceled {
		return taskserver.PostRunSubtask409JSONResponse{Message: "workflow run was canceled"}, nil
	}
	if req.StepKey == nil {
		return taskserver.PostRunSubtask400JSONResponse{Message: "step_key is required for a workflow's invoked task"}, nil
	}
	seq := req.StepKey.Seq

	if step, ok := a.coordinator.store.GetStep(a.runID, seq); ok {
		if step.Type != store.StepTypeInvoke || step.ChildRunId == nil {
			return taskserver.PostRunSubtask400JSONResponse{Message: fmt.Sprintf("step %d is not an invoked task", seq)}, nil
		}
		return taskserver.PostRunSubtask200JSONResponse{TaskRunId: *step.ChildRunId}, nil
	}

	task := a.coordinator.store.GetTaskByName(req.TaskName)
	if task == nil {
		return taskserver.PostRunSubtask404JSONResponse{Message: fmt.Sprintf("task %q not found", req.TaskName)}, nil
	}
	var input []byte
	if req.Input != nil {
		input = *req.Input
	}
	child, err := a.coordinator.StartTask(a.coordinator.topLevelContext, task.ID, input, parent)
	if err != nil {
		return taskserver.PostRunSubtask500Response{}, fmt.Errorf("error running subtask: %w", err)
	}

	step, _ := a.coordinator.store.StartStep(a.runID, taskserver.StepRecord{
		StepKey:    *req.StepKey,
		Type:       store.StepTypeInvoke,
		TaskName:   pointers.From(req.TaskName),
		Input:      req.Input,
		ChildRunId: pointers.From(child.ID),
		StartedAt:  a.coordinator.store.Now(),
	})
	return taskserver.PostRunSubtask200JSONResponse{TaskRunId: *step.ChildRunId}, nil
}

// pauseWorkflow pauses a run whose attempt suspended, and wakes it if its
// child already finished. The attempt's process is already stopped.
func (c *Coordinator) pauseWorkflow(taskRun *store.TaskRun, sig suspendSignal) {
	updated, err := c.store.PauseTaskRun(taskRun.ID)
	if err != nil {
		// Canceled while the attempt was suspending.
		return
	}

	c.statusReporter.TaskPaused(updated, fmt.Sprintf("waiting on %s", sig.childRunID))
	// The child may have finished before the run was paused, in which case
	// wakeWaitingParent saw a running parent and did nothing.
	if child := c.store.GetTaskRun(sig.childRunID); child != nil {
		if _, _, done := c.subtaskOutcome(child); done {
			c.wake(taskRun.ID)
		}
	}
}

// wake starts the next attempt of a paused workflow run. It's a no-op if
// the run was canceled or another wake got there first.
func (c *Coordinator) wake(taskRunID string) {
	taskRun, err := c.store.WakeTaskRun(taskRunID)
	if err != nil {
		return
	}
	if _, err := c.launchTask(taskRun); err != nil {
		fmt.Printf("failed to wake workflow run %s: %s\n", taskRunID, err)
	}
}

// wakeWaitingParent wakes the workflow run paused on child, if any. Call it
// after the child reaches its final state.
func (c *Coordinator) wakeWaitingParent(child *store.TaskRun) {
	if child.ParentTaskRunID == nil {
		return
	}
	parentID, _, ok := c.store.FindInvokeStep(child.ID)
	if !ok {
		return
	}
	if parent := c.store.GetTaskRun(parentID); parent != nil && parent.Status == store.TaskRunStatusPaused {
		c.wake(parentID)
	}
}

// recoverWorkflow replays a workflow run whose attempt crashed without a
// callback or reported a system error, until the recovery budget runs out.
func (c *Coordinator) recoverWorkflow(taskRun *store.TaskRun, exitErr error) {
	if taskRun.Recoveries >= maxRecoveries {
		errMsg := fmt.Sprintf("workflow attempt exited before completing; recovery budget of %d exhausted", maxRecoveries)
		if exitErr != nil {
			errMsg = fmt.Sprintf("%s: %s", errMsg, exitErr)
		}
		updated, err := c.store.FailTaskRun(taskRun.ID, errMsg)
		if err != nil {
			fmt.Println("error failing task", err)
			return
		}
		c.statusReporter.TaskFailed(updated)
		c.wakeWaitingParent(updated)
		return
	}

	updated, err := c.store.RecoverTaskRun(taskRun.ID)
	if err != nil {
		return
	}
	c.statusReporter.TaskReplaying(updated)

	select {
	case <-time.After(c.recoveryDelay):
	case <-c.topLevelContext.Done():
		return
	}
	if _, err := c.launchTask(updated); err != nil {
		fmt.Printf("failed to recover workflow run %s: %s\n", taskRun.ID, err)
	}
}

func isSystemError(taskErr *taskserver.TaskError) bool {
	return taskErr.Reason != nil && *taskErr.Reason == taskserver.TaskErrorReasonSystemError
}

// failureDetails formats a failure callback, spelling out determinism and
// limit errors.
func failureDetails(taskErr *taskserver.TaskError) string {
	if taskErr.Reason == nil {
		return taskErr.Details
	}
	switch reason := *taskErr.Reason; reason {
	case taskserver.TaskErrorReasonDeterminismError:
		if d := taskErr.Determinism; d != nil {
			return fmt.Sprintf("determinism error at step %s (kind %s): expected %s, got %s",
				formatStepKey(d.StepKey), d.Kind, pointers.StringValue(d.Expected), pointers.StringValue(d.Actual))
		}
	case taskserver.TaskErrorReasonWorkflowStepLimitExceeded,
		taskserver.TaskErrorReasonWorkflowPayloadTooLarge,
		taskserver.TaskErrorReasonWorkflowHistoryTooLarge:
		if taskErr.Limit != nil && taskErr.Value != nil {
			return fmt.Sprintf("%s (%d, limit %d): %s", reason, *taskErr.Value, *taskErr.Limit, taskErr.Details)
		}
	}
	return taskErr.Details
}

// formatStepKey names a step by its seq, plus its sub_key when the SDK sets one.
func formatStepKey(key taskserver.StepKey) string {
	if subKey := pointers.StringValue(key.SubKey); subKey != "" {
		return fmt.Sprintf("%d/%s", key.Seq, subKey)
	}
	return fmt.Sprintf("%d", key.Seq)
}

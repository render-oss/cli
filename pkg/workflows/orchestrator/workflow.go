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
	// It's a local-only guard so a crash-looping SDK eventually fails the run
	// instead of spinning; the platform replays without limit.
	maxRecoveries = 100
	// defaultRecoveryDelay keeps a crash-looping SDK from spinning.
	defaultRecoveryDelay = 500 * time.Millisecond
)

// workflowAttempt serves the step routes for one attempt of a workflow run.
type workflowAttempt struct {
	coordinator *Coordinator
	runID       string
	attempt     int
}

func newWorkflowAttempt(c *Coordinator, taskRun *store.TaskRun) *workflowAttempt {
	return &workflowAttempt{
		coordinator: c,
		runID:       taskRun.ID,
		attempt:     taskRun.WorkflowAttempt,
	}
}

func (a *workflowAttempt) cancelled() bool {
	taskRun := a.coordinator.store.GetTaskRun(a.runID)
	return taskRun == nil || taskRun.Status == store.TaskRunStatusCanceled
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

// StepWait answers 400 until local dev supports detached tasks.
func (a *workflowAttempt) StepWait(_ context.Context, _ taskserver.WaitRequest) (taskserver.PostStepWaitResponseObject, error) {
	return taskserver.PostStepWait400Response{}, nil
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

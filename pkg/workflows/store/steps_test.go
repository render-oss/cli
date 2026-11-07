package store_test

import (
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/workflows/store"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
	"github.com/stretchr/testify/require"
)

var stepClock = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

func newStepStore() *store.TaskStore {
	return store.NewTaskStore(store.WithNow(func() time.Time { return stepClock }))
}

func inlineStart(seq int, taskName string, input string) taskserver.StepRecord {
	return taskserver.StepRecord{
		StepKey:   taskserver.StepKey{Seq: seq},
		Type:      store.StepTypeInline,
		TaskName:  pointers.From(taskName),
		Input:     pointers.From([]byte(input)),
		StartedAt: stepClock,
	}
}

func failedAttempt(seq, attempt int, message string) taskserver.StepAttemptRecord {
	return taskserver.StepAttemptRecord{
		StepKey: taskserver.StepKey{Seq: seq},
		Attempt: attempt,
		Error:   taskserver.StepError{Name: "Error", Message: message},
	}
}

func failed(message string) store.StepCompletion {
	return store.StepCompletion{Error: &taskserver.StepError{Name: "Error", Message: message}}
}

func TestStartStep(t *testing.T) {
	s := newStepStore()

	start := inlineStart(0, "charge", `["a"]`)
	start.WorkflowAttempt = 3
	first, applied := s.StartStep("trn-1", start)
	require.True(t, applied)
	require.Equal(t, "charge", *first.TaskName)
	require.Equal(t, 0, first.WorkflowAttempt, "a step starts in workflow attempt 0")

	second, applied := s.StartStep("trn-1", inlineStart(0, "refund", `["b"]`))
	require.False(t, applied, "the first writer wins")
	require.Equal(t, "charge", *second.TaskName)
	require.Equal(t, []byte(`["a"]`), *second.Input)

	_, applied = s.StartStep("trn-2", inlineStart(0, "refund", `["b"]`))
	require.True(t, applied, "rows are scoped to their run")
}

func TestRecordStepAttempt(t *testing.T) {
	s := newStepStore()
	s.StartStep("trn-1", inlineStart(0, "charge", `[]`))

	recorded, applied := s.RecordStepAttempt("trn-1", failedAttempt(0, 0, "boom"))
	require.True(t, applied)
	require.Equal(t, stepClock, recorded.EndedAt, "ended_at is set by the server")

	existing, applied := s.RecordStepAttempt("trn-1", failedAttempt(0, 0, "again"))
	require.False(t, applied)
	require.Equal(t, "boom", existing.Error.Message)

	_, applied = s.RecordStepAttempt("trn-1", failedAttempt(0, 1, "again"))
	require.True(t, applied)
}

func TestCompleteStep(t *testing.T) {
	tests := []struct {
		name        string
		start       bool
		completions []store.StepCompletion
		wantApplied bool
		wantErr     error
		wantMessage string
	}{
		{
			name:        "completes a started row",
			start:       true,
			completions: []store.StepCompletion{failed("first")},
			wantApplied: true,
			wantMessage: "first",
		},
		{
			name:        "a second complete returns the winner",
			start:       true,
			completions: []store.StepCompletion{failed("first"), failed("second")},
			wantApplied: false,
			wantMessage: "first",
		},
		{
			name:        "complete without a start",
			completions: []store.StepCompletion{failed("first")},
			wantErr:     store.ErrMissingStart,
		},
		{
			name:  "complete before the start",
			start: true,
			completions: []store.StepCompletion{{
				CompletedAt: stepClock.Add(-time.Second),
				Error:       &taskserver.StepError{Name: "Error", Message: "first"},
			}},
			wantErr: store.ErrCompletedBeforeStart,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStepStore()
			if tt.start {
				s.StartStep("trn-1", inlineStart(0, "charge", `[]`))
			}

			var step taskserver.StepRecord
			var applied bool
			var err error
			for _, completion := range tt.completions {
				step, applied, err = s.CompleteStep("trn-1", 0, completion)
			}

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantApplied, applied)
			require.Equal(t, tt.wantMessage, step.Error.Message)
			require.Equal(t, stepClock, *step.CompletedAt)
		})
	}
}

func TestCompleteStepRecordsCompletingAttempt(t *testing.T) {
	s := newStepStore()
	s.StartStep("trn-1", inlineStart(0, "charge", `[]`))
	completedAt := stepClock.Add(time.Minute)

	step, applied, err := s.CompleteStep("trn-1", 0, store.StepCompletion{
		WorkflowAttempt: 2,
		CompletedAt:     completedAt,
		Output:          pointers.From([]byte(`["ok"]`)),
	})
	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, 2, step.WorkflowAttempt)
	require.Equal(t, completedAt, *step.CompletedAt, "the supplied completed_at is kept")
}

func TestCompleteStepFinalAttempt(t *testing.T) {
	final := func(message string) *taskserver.StepAttemptRecord {
		rec := failedAttempt(0, 1, message)
		rec.EndedAt = stepClock
		return &rec
	}
	withFinal := func(message string) store.StepCompletion {
		c := failed(message)
		c.FinalAttempt = final(message)
		return c
	}

	t.Run("records the final attempt with the outcome", func(t *testing.T) {
		s := newStepStore()
		s.StartStep("trn-1", inlineStart(0, "charge", `[]`))
		s.RecordStepAttempt("trn-1", failedAttempt(0, 0, "attempt 0"))

		_, applied, err := s.CompleteStep("trn-1", 0, withFinal("attempt 1"))
		require.NoError(t, err)
		require.True(t, applied)

		_, applied = s.RecordStepAttempt("trn-1", failedAttempt(0, 1, "late"))
		require.False(t, applied, "the final attempt is already recorded")
	})

	t.Run("an identical final attempt is a retry", func(t *testing.T) {
		s := newStepStore()
		s.StartStep("trn-1", inlineStart(0, "charge", `[]`))
		s.RecordStepAttempt("trn-1", *final("attempt 1"))

		step, applied, err := s.CompleteStep("trn-1", 0, withFinal("attempt 1"))
		require.NoError(t, err)
		require.True(t, applied)
		require.NotNil(t, step.CompletedAt)
	})

	t.Run("a conflicting final attempt leaves the step open", func(t *testing.T) {
		s := newStepStore()
		s.StartStep("trn-1", inlineStart(0, "charge", `[]`))
		s.RecordStepAttempt("trn-1", *final("recorded"))

		step, applied, err := s.CompleteStep("trn-1", 0, withFinal("different"))
		require.NoError(t, err)
		require.False(t, applied)
		require.Nil(t, step.CompletedAt)
	})
}

func TestHistory(t *testing.T) {
	s := newStepStore()

	s.StartStep("trn-1", inlineStart(1, "second", `[]`))
	s.StartStep("trn-1", inlineStart(0, "first", `[]`))
	s.StartStep("trn-1", inlineStart(2, "third", `[]`))
	s.RecordStepAttempt("trn-1", failedAttempt(0, 0, "seq 0 failed"))
	_, _, err := s.CompleteStep("trn-1", 0, store.StepCompletion{Output: pointers.From([]byte(`["ok"]`))})
	require.NoError(t, err)
	s.RecordStepAttempt("trn-1", failedAttempt(1, 1, "seq 1, attempt 1"))
	s.RecordStepAttempt("trn-1", failedAttempt(1, 0, "seq 1, attempt 0"))
	s.RecordStepAttempt("trn-1", failedAttempt(2, 0, "seq 2 failed"))

	history := s.History("trn-1")

	var seqs []int
	for _, step := range *history.Steps {
		seqs = append(seqs, step.StepKey.Seq)
	}
	require.Equal(t, []int{0, 1, 2}, seqs)

	// Only the lowest seq with no outcome contributes attempts, in order.
	var messages []string
	for _, attempt := range *history.StepAttempts {
		messages = append(messages, attempt.Error.Message)
	}
	require.Equal(t, []string{"seq 1, attempt 0", "seq 1, attempt 1"}, messages)

	empty := s.History("trn-unknown")
	require.Empty(t, *empty.Steps)
	require.Empty(t, *empty.StepAttempts)
}

func TestWorkflowRunTransitions(t *testing.T) {
	s := newStepStore()
	s.SetTasks([]taskserver.Task{{Name: "wf", Workflow: pointers.From(true)}})
	run := s.StartTaskRun("wf", nil, nil)
	require.True(t, run.IsWorkflow)

	recovered, err := s.RecoverTaskRun(run.ID)
	require.NoError(t, err)
	require.Equal(t, 1, recovered.WorkflowAttempt)
	require.Equal(t, 1, recovered.Recoveries)
	_, err = s.RecoverTaskRun(run.ID)
	require.Error(t, err, "only a running attempt can crash")

	_, err = s.MarkRunningTaskRun(run.ID)
	require.NoError(t, err)
	recovered, err = s.RecoverTaskRun(run.ID)
	require.NoError(t, err)
	require.Equal(t, 2, recovered.WorkflowAttempt)
	require.Equal(t, 2, recovered.Recoveries)

	_, err = s.CancelTaskRun(run.ID)
	require.NoError(t, err)
	_, err = s.MarkRunningTaskRun(run.ID)
	require.Error(t, err, "a canceled run never starts another attempt")
}

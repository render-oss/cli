package store

import (
	"errors"
	"reflect"
	"slices"
	"time"

	"github.com/render-oss/cli/pkg/workflows/taskserver"
)

// Step types recorded in a workflow's history.
const (
	StepTypeInline = "inline"
)

var (
	// ErrMissingStart is returned when a step is completed before it was started.
	ErrMissingStart = errors.New("step has no start row")
	// ErrCompletedBeforeStart is returned when an outcome's completed_at is
	// earlier than the step's started_at.
	ErrCompletedBeforeStart = errors.New("step completed before it started")
)

// StepCompletion is the outcome that completes a step.
type StepCompletion struct {
	// WorkflowAttempt is the workflow attempt completing the step.
	WorkflowAttempt int
	// CompletedAt defaults to the store's clock when zero.
	CompletedAt time.Time
	Output      *[]byte
	Error       *taskserver.StepError
	// FinalAttempt is the failed attempt that ended an inline step. It is
	// recorded together with the outcome, or not at all.
	FinalAttempt *taskserver.StepAttemptRecord
}

// StartStep writes a step's start row. The first writer wins: when a row
// already exists for the seq, it is returned unchanged with applied false.
// A step starts in workflow attempt 0; CompleteStep sets the attempt that
// completed it.
func (s *TaskStore) StartStep(taskRunID string, rec taskserver.StepRecord) (taskserver.StepRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	runSteps, ok := s.steps[taskRunID]
	if !ok {
		runSteps = make(map[int]*taskserver.StepRecord)
		s.steps[taskRunID] = runSteps
	}
	if existing, ok := runSteps[rec.StepKey.Seq]; ok {
		return *existing, false
	}

	rec.CompletedAt = nil
	rec.WorkflowAttempt = 0
	runSteps[rec.StepKey.Seq] = &rec
	return rec, true
}

// RecordStepAttempt records a failed inline attempt with a server-set
// ended_at. It returns false when the (seq, attempt) row already exists.
func (s *TaskStore) RecordStepAttempt(taskRunID string, rec taskserver.StepAttemptRecord) (taskserver.StepAttemptRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.findStepAttempt(taskRunID, rec.StepKey.Seq, rec.Attempt); ok {
		return existing, false
	}

	rec.EndedAt = s.now()
	s.stepAttempts[taskRunID] = append(s.stepAttempts[taskRunID], rec)
	return rec, true
}

// CompleteStep sets a step's outcome in place. The first writer wins: when
// the row is already completed it is returned unchanged with applied false.
// A final attempt that conflicts with a recorded one also returns applied
// false, leaving the step open; an identical one is a retry and applies.
func (s *TaskStore) CompleteStep(taskRunID string, seq int, c StepCompletion) (taskserver.StepRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	row, ok := s.steps[taskRunID][seq]
	if !ok {
		return taskserver.StepRecord{}, false, ErrMissingStart
	}
	if row.CompletedAt != nil {
		return *row, false, nil
	}
	completedAt := c.CompletedAt
	if completedAt.IsZero() {
		completedAt = s.now()
	}
	if completedAt.Before(row.StartedAt) {
		return taskserver.StepRecord{}, false, ErrCompletedBeforeStart
	}

	if final := c.FinalAttempt; final != nil {
		existing, ok := s.findStepAttempt(taskRunID, seq, final.Attempt)
		if ok && !reflect.DeepEqual(existing, *final) {
			return *row, false, nil
		}
		if !ok {
			s.stepAttempts[taskRunID] = append(s.stepAttempts[taskRunID], *final)
		}
	}

	row.CompletedAt = &completedAt
	row.WorkflowAttempt = c.WorkflowAttempt
	row.Output = c.Output
	row.Error = c.Error
	return *row, true, nil
}

func (s *TaskStore) findStepAttempt(taskRunID string, seq, attempt int) (taskserver.StepAttemptRecord, bool) {
	for _, existing := range s.stepAttempts[taskRunID] {
		if existing.StepKey.Seq == seq && existing.Attempt == attempt {
			return existing, true
		}
	}
	return taskserver.StepAttemptRecord{}, false
}

// GetStep returns the step row for a seq.
func (s *TaskStore) GetStep(taskRunID string, seq int) (taskserver.StepRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	row, ok := s.steps[taskRunID][seq]
	if !ok {
		return taskserver.StepRecord{}, false
	}
	return *row, true
}

// History returns every step row of a run in seq order, plus the failed
// attempts of the lowest seq that has no outcome yet.
func (s *TaskStore) History(taskRunID string) taskserver.History {
	s.mu.Lock()
	defer s.mu.Unlock()

	steps := make([]taskserver.StepRecord, 0, len(s.steps[taskRunID]))
	openSeq := -1
	for _, row := range s.steps[taskRunID] {
		steps = append(steps, *row)
	}
	slices.SortFunc(steps, func(a, b taskserver.StepRecord) int { return a.StepKey.Seq - b.StepKey.Seq })
	for _, step := range steps {
		if step.CompletedAt == nil {
			openSeq = step.StepKey.Seq
			break
		}
	}

	attempts := make([]taskserver.StepAttemptRecord, 0)
	for _, attempt := range s.stepAttempts[taskRunID] {
		if attempt.StepKey.Seq == openSeq {
			attempts = append(attempts, attempt)
		}
	}
	slices.SortFunc(attempts, func(a, b taskserver.StepAttemptRecord) int { return a.Attempt - b.Attempt })

	return taskserver.History{Steps: &steps, StepAttempts: &attempts}
}

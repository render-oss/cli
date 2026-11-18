package flowrun

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	workflows "github.com/render-oss/cli/pkg/client/workflows"
)

type StepAttempt struct {
	Attempt     int64                       `json:"attempt" yaml:"attempt"`
	Recorded    bool                        `json:"recorded" yaml:"recorded"`
	Status      workflows.FlowRunStepStatus `json:"status" yaml:"status"`
	FlowAttempt *int64                      `json:"flowAttempt,omitempty" yaml:"flowAttempt,omitempty"`
	StartedAt   *time.Time                  `json:"startedAt,omitempty" yaml:"startedAt,omitempty"`
	CompletedAt *time.Time                  `json:"completedAt,omitempty" yaml:"completedAt,omitempty"`
	RetryAt     *time.Time                  `json:"retryAt,omitempty" yaml:"retryAt,omitempty"`
	Error       *string                     `json:"error,omitempty" yaml:"error,omitempty"`
	Input       interface{}                 `json:"input,omitempty" yaml:"input,omitempty"`
	Result      *json.RawMessage            `json:"result,omitempty" yaml:"result,omitempty"`
}

func (attempt StepAttempt) MarshalJSON() ([]byte, error) {
	type stepAttempt StepAttempt
	output := stepAttempt(attempt)
	output.Attempt++
	if output.FlowAttempt != nil {
		flowAttempt := *output.FlowAttempt + 1
		output.FlowAttempt = &flowAttempt
	}
	return json.Marshal(output)
}

type Service struct{ repo *Repo }

func NewService(repo *Repo) *Service { return &Service{repo: repo} }

func (s *Service) ListStepAttempts(ctx context.Context, runID string, seq int, key string) ([]StepAttempt, error) {
	step, err := s.repo.GetFlowRunStepWithAttempts(ctx, runID, seq, key)
	if err != nil {
		return nil, err
	}
	return stepAttempts(step)
}

func (s *Service) GetStepAttempt(ctx context.Context, runID string, seq int, key string, attempt int64) (*StepAttempt, error) {
	step, err := s.repo.GetFlowRunStepWithAttempts(ctx, runID, seq, key)
	if err != nil {
		return nil, err
	}
	attempts, err := stepAttempts(step)
	if err != nil {
		return nil, err
	}
	for _, row := range attempts {
		if row.Attempt != attempt {
			continue
		}
		if row.Recorded {
			detail, err := s.repo.GetFlowRunStepAttempt(ctx, runID, seq, key, attempt)
			if err != nil {
				return nil, err
			}
			row.Error = &detail.Error
		} else {
			row.Error = step.Error
			row.Input = step.Input
			row.Result = step.Result
		}
		return &row, nil
	}
	return nil, fmt.Errorf("step attempt %d not found", attempt+1)
}

func stepAttempts(step *workflows.FlowRunStepDetails) ([]StepAttempt, error) {
	if step.Type != workflows.FlowRunStepTypeInline {
		return nil, fmt.Errorf("step attempts are only supported for inline steps (step type: %s)", step.Type)
	}
	if step.FailedAttempts == nil {
		return nil, fmt.Errorf("this API does not support step attempts: the response omitted failedAttempts; update the API before using this command")
	}
	attempts := make([]StepAttempt, 0, len(*step.FailedAttempts)+1)
	var next int64
	for _, recorded := range *step.FailedAttempts {
		attempts = append(attempts, StepAttempt{Attempt: recorded.Attempt, Recorded: true, Status: workflows.FlowRunStepStatusFailed, FlowAttempt: &recorded.FlowAttempt, StartedAt: &recorded.StartedAt, CompletedAt: &recorded.CompletedAt, RetryAt: recorded.RetryAt})
		if recorded.Attempt >= next {
			next = recorded.Attempt + 1
		}
	}
	sort.Slice(attempts, func(i, j int) bool { return attempts[i].Attempt < attempts[j].Attempt })
	if len(attempts) == 0 || step.Status == workflows.FlowRunStepStatusRunning || step.Status == workflows.FlowRunStepStatusSucceeded {
		if next > math.MaxUint32 {
			return nil, fmt.Errorf("step outcome attempt exceeds the supported attempt range")
		}
		attempts = append(attempts, StepAttempt{Attempt: next, Recorded: false, Status: step.Status, CompletedAt: step.CompletedAt})
	}
	return attempts, nil
}

func StepAttemptDetails(attempt *StepAttempt) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Attempt: %d\nStatus: %s\nRecorded: %t\n", attempt.Attempt+1, attempt.Status, attempt.Recorded)
	if attempt.FlowAttempt != nil {
		fmt.Fprintf(&b, "Flow attempt: %d\n", *attempt.FlowAttempt+1)
	}
	if attempt.StartedAt != nil {
		fmt.Fprintf(&b, "Started at: %s\n", attempt.StartedAt.Format(time.RFC3339))
	}
	if attempt.CompletedAt != nil {
		fmt.Fprintf(&b, "Completed at: %s\n", attempt.CompletedAt.Format(time.RFC3339))
	}
	if attempt.RetryAt != nil {
		fmt.Fprintf(&b, "Retry at: %s\n", attempt.RetryAt.Format(time.RFC3339))
	}
	if attempt.Error != nil {
		fmt.Fprintf(&b, "Error: %s\n", *attempt.Error)
	}
	if attempt.Input != nil {
		if input, err := json.Marshal(attempt.Input); err == nil {
			fmt.Fprintf(&b, "Input: %s\n", input)
		}
	}
	if attempt.Result != nil {
		fmt.Fprintf(&b, "Result: %s\n", *attempt.Result)
	}
	return b.String()
}

func StepAttemptTable(attempts []StepAttempt) string {
	var b strings.Builder
	for i := range attempts {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(StepAttemptDetails(&attempts[i]))
	}
	return b.String()
}

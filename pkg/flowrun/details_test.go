package flowrun_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	workflows "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/flowrun"
	"github.com/render-oss/cli/pkg/pointers"
)

func TestErrorDescription(t *testing.T) {
	tcs := []struct {
		name    string
		details workflows.FlowRunErrorDetails
		want    string
	}{
		{
			name: "determinism error names the step and what changed",
			details: workflows.FlowRunErrorDetails{
				Code:        workflows.FlowDeterminismError,
				Message:     "replay diverged",
				Kind:        pointers.From(workflows.InputHash),
				Seq:         pointers.From(4),
				SubKey:      pointers.From("followup"),
				FlowAttempt: pointers.From(2),
				Expected:    pointers.From("sha256:aaa"),
				Actual:      pointers.From("sha256:bbb"),
			},
			want: "Error: Determinism error at step 4 (followup): the step's name or input changed since the original run\n" +
				"  Recorded by attempt: 3\n" +
				"  Expected: sha256:aaa\n" +
				"  Actual: sha256:bbb\n",
		},
		{
			name: "determinism error without a sub key or kind",
			details: workflows.FlowRunErrorDetails{
				Code:    workflows.FlowDeterminismError,
				Message: "replay diverged",
				Seq:     pointers.From(7),
			},
			want: "Error: Determinism error at step 7: the replay didn't match the run's history\n",
		},
		{
			name: "step limit shows the limit and the run's value",
			details: workflows.FlowRunErrorDetails{
				Code:    workflows.FlowStepLimitExceeded,
				Message: "step limit exceeded",
				Limit:   pointers.From(int64(10_000)),
				Value:   pointers.From(int64(10_001)),
			},
			want: "Error: Step limit reached: the run started too many steps, or one step used up its attempts\n" +
				"  Limit: 10,000\n" +
				"  Value: 10,001\n",
		},
		{
			name: "payload limit shows exact bytes",
			details: workflows.FlowRunErrorDetails{
				Code:    workflows.FlowPayloadTooLarge,
				Message: "payload too large",
				Limit:   pointers.From(int64(256_000)),
				Value:   pointers.From(int64(1_843_200)),
			},
			want: "Error: Step payload too large: a step's input, output, or error is over the size limit in bytes\n" +
				"  Limit: 256,000 bytes\n" +
				"  Value: 1,843,200 bytes\n",
		},
		{
			name: "history limit shows exact bytes",
			details: workflows.FlowRunErrorDetails{
				Code:    workflows.FlowHistoryTooLarge,
				Message: "history too large",
				Limit:   pointers.From(int64(10_000_000)),
				Value:   pointers.From(int64(10_000_001)),
			},
			want: "Error: Run history too large: the run's recorded history is over the size limit in bytes\n" +
				"  Limit: 10,000,000 bytes\n" +
				"  Value: 10,000,001 bytes\n",
		},
		{
			name: "history limit without measurements",
			details: workflows.FlowRunErrorDetails{
				Code:    workflows.FlowHistoryTooLarge,
				Message: "history too large",
			},
			want: "Error: Run history too large: the run's recorded history is over the size limit in bytes\n",
		},
		{
			name: "unknown code falls back to the message",
			details: workflows.FlowRunErrorDetails{
				Code:    "flow_something_new",
				Message: "something new went wrong",
			},
			want: "Error: something new went wrong\n",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, flowrun.ErrorDescription(&tc.details))
		})
	}
}

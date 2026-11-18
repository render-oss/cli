package flowrun_test

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/evertras/bubble-table/table"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	workflows "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/flowrun"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/style"
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

func TestIsFinished(t *testing.T) {
	tests := []struct {
		status workflows.FlowRunStatus
		want   bool
	}{
		{workflows.FlowRunStatusPending, false},
		{workflows.FlowRunStatusRunning, false},
		{workflows.FlowRunStatusPaused, false},
		{workflows.FlowRunStatusSucceeded, true},
		{workflows.FlowRunStatusFailed, true},
		{workflows.FlowRunStatusCanceled, true},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			assert.Equal(t, tc.want, flowrun.IsFinished(tc.status))
		})
	}
}

func TestRunStatusPresentation(t *testing.T) {
	for _, tc := range []struct {
		raw         string
		label       string
		statusStyle lipgloss.Style
	}{
		{"pending", "Queued", style.Status.Foreground(style.ColorDeprioritized)},
		{"running", "In Progress", style.Status.Foreground(style.ColorInfo)},
		{"sleeping", "Sleeping", style.Status.Foreground(style.ColorWarningDeprioritized)},
		{"succeeded", "Succeeded", style.Status.Foreground(style.ColorOK)},
		{"completed", "Succeeded", style.Status.Foreground(style.ColorOK)},
		{"failed", "Failed", style.Status.Foreground(style.ColorError)},
		{"canceled", "Canceled", style.Status.Foreground(style.ColorDeprioritized)},
		{"paused", "In Progress", style.Status.Foreground(style.ColorInfo)},
		{"future_state", "future_state", style.Status.Foreground(style.ColorDeprioritized)},
		{"suspended", "suspended", style.Status.Foreground(style.ColorDeprioritized)},
		{"", "", style.Status.Foreground(style.ColorDeprioritized)},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			run := &workflows.FlowRun{Id: "run-1", Status: workflows.FlowRunStatus(tc.raw)}
			original := *run
			row := flowrun.RunTableRow(run)
			cell, ok := row.Data["Status"].(table.StyledCell)
			require.True(t, ok)
			assert.Equal(t, tc.label, fmt.Sprintf("%v", cell.Data))
			assert.Equal(t, tc.statusStyle.GetForeground(), cell.Style.GetForeground())
			assert.Same(t, run, row.Data["flowRun"])
			assert.Equal(t, original, *run)
			details := &workflows.FlowRunDetails{Id: run.Id, Status: run.Status}
			originalDetails := *details
			var status string
			for _, kv := range flowrun.RunDetailsFormat(details) {
				if kv.Key == "Status" {
					status = ansi.Strip(kv.Value)
				}
			}
			assert.Equal(t, tc.label, status)
			assert.Equal(t, originalDetails, *details)
		})
	}
}

func TestFlowRunTableFilterMatchesStatusLabelAndRawStatus(t *testing.T) {
	for _, tc := range []struct {
		status  workflows.FlowRunStatus
		filters []string
	}{
		{workflows.FlowRunStatusPending, []string{"queued", "pending"}},
		{workflows.FlowRunStatusRunning, []string{"progress", "running"}},
		{workflows.FlowRunStatusPaused, []string{"progress", "paused"}},
	} {
		for _, filter := range tc.filters {
			t.Run(string(tc.status)+"/"+filter, func(t *testing.T) {
				model := table.New(flowrun.RunColumns()).
					WithRows([]table.Row{
						flowrun.RunTableRow(&workflows.FlowRun{Id: "matching", Status: tc.status}),
						flowrun.RunTableRow(&workflows.FlowRun{Id: "nonmatching", Status: workflows.FlowRunStatusFailed}),
					}).
					Filtered(true).
					WithFilterInputValue(filter)
				rows := model.GetVisibleRows()
				require.Len(t, rows, 1)
				assert.Equal(t, "matching", rows[0].Data["ID"])
			})
		}
	}
}

func TestFlowRunTableRendersOnlyStatusLabel(t *testing.T) {
	model := table.New(flowrun.RunColumns()).
		WithRows([]table.Row{flowrun.RunTableRow(&workflows.FlowRun{Id: "run-1", Status: workflows.FlowRunStatusPending})}).
		WithTargetWidth(100)
	view := ansi.Strip(model.View())
	assert.Contains(t, view, "Queued")
	assert.NotContains(t, view, "pending")
}

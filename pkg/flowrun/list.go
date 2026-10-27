package flowrun

import (
	"strconv"
	"time"

	"github.com/charmbracelet/lipgloss"

	workflows "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/style"
)

// FlowHeader returns the column titles for a flow list table.
func FlowHeader() []string {
	return []string{"Name", "ID", "Workflow ID", "Version ID", "Created"}
}

// FlowRow returns the row values for a single flow in a list table.
func FlowRow(f *workflows.Flow) []string {
	return []string{
		f.Name,
		f.Id,
		pointers.StringValue(f.WorkflowId),
		pointers.StringValue(f.WorkflowVersionId),
		pointers.TimeValue(&f.CreatedAt),
	}
}

// RunHeader returns the column titles for a flow run list table.
func RunHeader() []string {
	return []string{"ID", "Flow ID", "Status", "Started", "Completed", "Duration"}
}

// RunRow returns the row values for a single flow run in a list table.
func RunRow(run *workflows.FlowRun) []string {
	var started, completed, duration string
	if run.StartedAt != nil {
		started = run.StartedAt.Format(time.RFC3339)
		if run.CompletedAt != nil {
			completed = run.CompletedAt.Format(time.RFC3339)
			duration = run.CompletedAt.Sub(*run.StartedAt).String()
		}
	}
	return []string{
		run.Id,
		run.FlowId,
		string(run.Status),
		started,
		completed,
		duration,
	}
}

// StepHeader returns the column titles for a flow run step list table.
func StepHeader() []string {
	return []string{"Seq", "Name", "Type", "Status", "Started", "Completed", "Attempt"}
}

// StepRow returns the row values for a single flow run step in a list table.
func StepRow(step *workflows.FlowRunStep) []string {
	var completed string
	if step.CompletedAt != nil {
		completed = step.CompletedAt.Format(time.RFC3339)
	}
	return []string{
		strconv.Itoa(step.Seq),
		pointers.StringValue(step.Name),
		string(step.Type),
		string(step.Status),
		step.StartedAt.Format(time.RFC3339),
		completed,
		strconv.Itoa(step.FlowRunAttempt),
	}
}

func runStatusStyle(status workflows.FlowRunStatus) lipgloss.Style {
	switch status {
	case workflows.FlowRunStatusFailed, workflows.FlowRunStatusCanceled:
		return style.Status.Foreground(style.ColorError)
	case workflows.FlowRunStatusPending, workflows.FlowRunStatusRunning, workflows.FlowRunStatusPaused:
		return style.Status.Foreground(style.ColorWarning)
	}
	return style.Status.Foreground(style.ColorOK)
}

// RunStatusStyle exposes the status style so other packages can render matching badges.
func RunStatusStyle(status workflows.FlowRunStatus) lipgloss.Style {
	return runStatusStyle(status)
}

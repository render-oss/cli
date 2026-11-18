package taskrun

import (
	"time"

	"github.com/charmbracelet/lipgloss"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/tui"
)

func statusWithStyle(status wfclient.TaskRunStatus) lipgloss.Style {
	return tui.RunStatusStyle(string(status))
}

func Header() []string {
	return []string{"ID", "Status", "Started", "Completed", "Duration"}
}

func Row(taskRun *wfclient.TaskRun) []string {
	var started, completed, duration string

	if taskRun.StartedAt != nil {
		started = taskRun.StartedAt.Format(time.RFC3339)

		if taskRun.CompletedAt != nil {
			completed = taskRun.CompletedAt.Format(time.RFC3339)
			duration = taskRun.CompletedAt.Sub(*taskRun.StartedAt).String()
		}
	}

	return []string{
		taskRun.Id,
		string(taskRun.Status),
		started,
		completed,
		duration,
	}
}

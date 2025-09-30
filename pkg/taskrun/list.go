package taskrun

import (
	"time"

	"github.com/charmbracelet/lipgloss"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/style"
)

type ListItem struct {
	taskRun *wfclient.TaskRun
}

func (i ListItem) TaskRun() *wfclient.TaskRun {
	return i.taskRun
}

func NewListItem(tr *wfclient.TaskRun) ListItem {
	return ListItem{taskRun: tr}
}

func (i ListItem) Title() string {
	return style.Title.Render(i.taskRun.Id)
}

func statusWithStyle(status wfclient.TaskRunStatus) lipgloss.Style {
	switch status {
	case wfclient.Failed:
		return style.Status.Foreground(style.ColorError)
	case wfclient.Pending, wfclient.Running:
		return style.Status.Foreground(style.ColorWarning)
	}
	return style.Status.Foreground(style.ColorOK)
}

func (i ListItem) Description() string {
	statusLine := statusWithStyle(i.taskRun.Status).Render(string(i.taskRun.Status))

	var timeInfo string
	if i.taskRun.StartedAt != nil {
		if i.taskRun.CompletedAt != nil {
			duration := i.taskRun.CompletedAt.Sub(*i.taskRun.StartedAt)
			timeInfo = lipgloss.JoinHorizontal(lipgloss.Left,
				style.FormatKeyValue("Started", pointers.TimeValue(i.taskRun.StartedAt)),
				"   ",
				style.FormatKeyValue("Duration", duration.String()),
			)
		} else {
			timeInfo = style.FormatKeyValue("Started", pointers.TimeValue(i.taskRun.StartedAt))
		}
	} else {
		timeInfo = style.FormatKeyValue("Status", "Not started")
	}

	return lipgloss.JoinVertical(lipgloss.Left, statusLine, timeInfo)
}

func (i ListItem) FilterValue() string {
	return i.taskRun.Id
}

func (i ListItem) Height() int {
	return 5
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
		} else {
			completed = ""
			duration = ""
		}
	} else {
		started = ""
		completed = ""
		duration = ""
	}

	return []string{
		taskRun.Id,
		string(taskRun.Status),
		started,
		completed,
		duration,
	}
}

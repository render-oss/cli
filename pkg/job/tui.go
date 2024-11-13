package job

import (
	"github.com/charmbracelet/lipgloss"
	clientjob "github.com/renderinc/cli/pkg/client/jobs"
	"github.com/renderinc/cli/pkg/pointers"
	rstrings "github.com/renderinc/cli/pkg/strings"
	"github.com/renderinc/cli/pkg/style"
)

type ListItem struct {
	job *clientjob.Job
}

func NewListItem(j *clientjob.Job) ListItem {
	return ListItem{job: j}
}

func (i ListItem) Title() string {
	return style.Title.Render(i.job.Id)
}

func (i ListItem) Description() string {
	statusValue := jobStatusValue(i.job.Status)
	var status lipgloss.Style
	switch statusValue {
	case "Succeeded":
		status = style.Status.Foreground(style.ColorOK)
	case "Failed":
		status = style.Status.Foreground(style.ColorError)
	case "Canceled":
		status = style.Status.Foreground(style.ColorWarning)
	default:
		status = style.Status.Foreground(style.ColorDeprioritized)
	}

	statusLine := status.Render(statusValue)

	timeLine := lipgloss.JoinHorizontal(lipgloss.Left,
		style.FormatKeyValue("Started", pointers.TimeValue(i.job.StartedAt)),
		"   ",
		style.FormatKeyValue("Finished", pointers.TimeValue(i.job.FinishedAt)),
	)

	jobInfoLine := style.FormatKeyValue("Command", i.job.StartCommand) + " " +
		style.FormatKeyValue("Plan", i.job.PlanId)

	return lipgloss.JoinVertical(lipgloss.Left,
		statusLine,
		timeLine,
		jobInfoLine,
	)
}

func (i ListItem) Job() *clientjob.Job {
	return i.job
}

func (i ListItem) FilterValue() string {
	return i.job.Id
}

func (i ListItem) Height() int {
	return 5
}

func jobStatusValue(status *clientjob.JobStatus) string {
	if status == nil {
		return ""
	}

	statusStr := string(*status)
	return rstrings.TitleCaseValue(statusStr)
}

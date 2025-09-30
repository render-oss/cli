package task

import (
	"github.com/charmbracelet/lipgloss"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/style"
)

type ListItem struct {
	task *wfclient.Task
}

func (i ListItem) Task() *wfclient.Task {
	return i.task
}

func NewListItem(t *wfclient.Task) ListItem {
	return ListItem{task: t}
}

func (i ListItem) Title() string {
	return style.Title.Render(i.task.Name)
}

func (i ListItem) Description() string {
	statusLine := style.Status.Foreground(style.ColorDeprioritized).Render("Ready")

	timeLine := lipgloss.JoinHorizontal(lipgloss.Left,
		style.FormatKeyValue("ID", i.task.Id),
		"   ",
		style.FormatKeyValue("Created", pointers.TimeValue(&i.task.CreatedAt)),
	)

	return lipgloss.JoinVertical(lipgloss.Left, statusLine, timeLine)
}

func (i ListItem) FilterValue() string {
	return i.task.Name
}

func (i ListItem) Height() int {
	return 5
}

func Header() []string {
	return []string{"Name", "ID", "Created"}
}

func Row(task *wfclient.Task) []string {
	return []string{
		task.Name,
		task.Id,
		pointers.TimeValue(&task.CreatedAt),
	}
}

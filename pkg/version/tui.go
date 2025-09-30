package version

import (
	"github.com/charmbracelet/lipgloss"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/style"
)

type ListItem struct {
	version *wfclient.WorkflowVersion
}

func (i ListItem) Version() *wfclient.WorkflowVersion {
	return i.version
}

func NewListItem(v *wfclient.WorkflowVersion) ListItem {
	return ListItem{version: v}
}

func (i ListItem) Title() string {
	return style.Title.Render(i.version.Id)
}

func (i ListItem) Description() string {
	statusLine := style.Status.Foreground(style.ColorDeprioritized).Render("Unknown")

	timeLine := lipgloss.JoinHorizontal(lipgloss.Left,
		style.FormatKeyValue("Created", pointers.TimeValue(&i.version.CreatedAt)),
		"   ",
	)

	return lipgloss.JoinVertical(lipgloss.Left, statusLine, timeLine)
}

func (i ListItem) FilterValue() string {
	return i.version.Id
}

func (i ListItem) Height() int {
	return 5
}

func Header() []string {
	return []string{"ID", "Created"}
}

func Row(version *wfclient.WorkflowVersion) []string {
	return []string{
		version.Id,
		pointers.TimeValue(&version.CreatedAt),
	}
}

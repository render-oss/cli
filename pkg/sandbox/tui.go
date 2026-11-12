package sandbox

import (
	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/utils"
)

// ListItem renders a sandbox in a tui.List.
type ListItem struct {
	sandbox *sandboxclient.Sandbox
}

func NewListItem(s *sandboxclient.Sandbox) ListItem {
	return ListItem{sandbox: s}
}

func (i ListItem) Title() string {
	return style.Title.Render(i.sandbox.Id)
}

func (i ListItem) Description() string {
	return style.FormatKeyValue("Status", string(i.sandbox.Status)) + "   " +
		style.FormatKeyValue("Plan", string(i.sandbox.Plan)) + "   " +
		style.FormatKeyValue("Region", string(i.sandbox.Region)) + "   " +
		style.FormatKeyValue("Age", utils.FormatDuration(i.sandbox.CreatedAt))
}

func (i ListItem) FilterValue() string {
	// Filtering is currently disabled at the base list component
	return i.sandbox.Id
}

func (i ListItem) Height() int {
	return 2
}

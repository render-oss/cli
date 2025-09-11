package instance

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/utils"
)

type ListItem struct {
	instance *client.ServiceInstance
}

func (i ListItem) Instance() *client.ServiceInstance {
	return i.instance
}

func NewListItem(instance *client.ServiceInstance) ListItem {
	return ListItem{instance: instance}
}

func (i ListItem) Title() string {
	return style.Title.Render(i.instance.Id)
}

func (i ListItem) Description() string {
	age := utils.FormatDuration(i.instance.CreatedAt)
	ageLine := style.FormatKeyValue("Age", age)
	createdLine := style.FormatKeyValue("Created", i.instance.CreatedAt.Format("2006-01-02 15:04:05"))

	return lipgloss.JoinVertical(lipgloss.Left,
		ageLine,
		createdLine,
	)
}

func (i ListItem) FilterValue() string {
	return i.instance.Id
}

func (i ListItem) Height() int {
	return 3
}

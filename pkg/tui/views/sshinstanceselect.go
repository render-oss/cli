package views

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/instance"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/utils"
)

type SSHInstanceOption struct {
	ID        string
	Name      string
	CreatedAt *time.Time
}

func (o SSHInstanceOption) Title() string {
	return o.Name
}

func (o SSHInstanceOption) Description() string {
	if o.ID == "" {
		return "Connect to any available instance"
	}
	if o.CreatedAt != nil {
		age := utils.FormatDuration(*o.CreatedAt)
		return fmt.Sprintf("Age: %s", age)
	}
	return ""
}

func (o SSHInstanceOption) FilterValue() string {
	return o.Name + " " + o.ID
}

func (o SSHInstanceOption) Height() int {
	return 2
}

type SSHInstanceSelectionView struct {
	list *tui.List[SSHInstanceOption]
}

func NewSSHInstanceSelectionView(ctx context.Context, serviceID string, onSelect func(instanceID string) tea.Cmd) *SSHInstanceSelectionView {
	loadInstancesForSSH := func(ctx context.Context, serviceID string, cur client.Cursor) (client.Cursor, []SSHInstanceOption, error) {
		c, err := client.NewDefaultClient()
		if err != nil {
			return "", nil, fmt.Errorf("failed to create client: %w", err)
		}

		instanceRepo := instance.NewRepo(c)
		instances, err := instanceRepo.ListInstancesForService(ctx, serviceID)
		if err != nil {
			return "", nil, err
		}

		options := []SSHInstanceOption{
			{ID: "", Name: "Any instance"},
		}

		for _, inst := range instances {
			options = append(options, SSHInstanceOption{
				ID:        inst.Id,
				Name:      inst.Id,
				CreatedAt: &inst.CreatedAt,
			})
		}

		return "", options, nil
	}

	onSelectOption := func(selectedItem tui.ListItem) tea.Cmd {
		option := selectedItem.(SSHInstanceOption)
		return onSelect(option.ID)
	}

	list := tui.NewList(
		"Select an instance:",
		command.PaginatedLoadCmd(ctx, loadInstancesForSSH, serviceID),
		func(option SSHInstanceOption) tui.ListItem {
			return option
		},
		tui.WithOnSelect[SSHInstanceOption](onSelectOption),
	)

	return &SSHInstanceSelectionView{
		list: list,
	}
}

func (v *SSHInstanceSelectionView) Init() tea.Cmd {
	return v.list.Init()
}

func (v *SSHInstanceSelectionView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := v.list.Update(msg)
	return v, cmd
}

func (v *SSHInstanceSelectionView) View() string {
	return v.list.View()
}

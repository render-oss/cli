package views

import (
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/instance"
	"github.com/render-oss/cli/pkg/tui"
)

type InstanceListInput struct {
	ServiceID string `cli:"arg:0"`
}

func (in InstanceListInput) Validate(interactive bool) error {
	if !interactive && in.ServiceID == "" {
		return errors.New("service id must be specified when output is not interactive")
	}
	return nil
}

func LoadInstanceList(ctx context.Context, input InstanceListInput) ([]*client.ServiceInstance, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	instanceRepo := instance.NewRepo(c)
	return instanceRepo.ListInstancesForService(ctx, input.ServiceID)
}

type InstanceListView struct {
	list *tui.List[*client.ServiceInstance]
}

func NewInstanceListView(ctx context.Context, input InstanceListInput) *InstanceListView {
	list := tui.NewList(
		"",
		command.LoadCmd(ctx, LoadInstanceList, input),
		func(serviceInstance *client.ServiceInstance) tui.ListItem {
			return instance.NewListItem(serviceInstance)
		},
	)

	return &InstanceListView{
		list: list,
	}
}

func (v *InstanceListView) Init() tea.Cmd {
	return v.list.Init()
}

func (v *InstanceListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := v.list.Update(msg)
	return v, cmd
}

func (v *InstanceListView) View() string {
	return v.list.View()
}

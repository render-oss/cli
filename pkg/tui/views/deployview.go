package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/deploy"
	"github.com/renderinc/cli/pkg/tui"
)

type DeployListInput struct {
	ServiceID string `cli:"arg:0"`
}

func LoadDeployList(ctx context.Context, input DeployListInput) ([]*client.Deploy, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := c.ListDeploysWithResponse(ctx, input.ServiceID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list deploys: %w", err)
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %s", resp.Status())
	}

	deploys := make([]*client.Deploy, len(*resp.JSON200))
	for i, d := range *resp.JSON200 {
		deploys[i] = d.Deploy
	}

	return deploys, nil
}

type DeployListView struct {
	list *tui.List[*client.Deploy]
}

func NewDeployListView(ctx context.Context, input DeployListInput) *DeployListView {
	list := tui.NewList(
		"",
		command.LoadCmd(ctx, LoadDeployList, input),
		func(d *client.Deploy) tui.ListItem {
			return deploy.NewListItem(d)
		},
	)

	return &DeployListView{
		list: list,
	}
}

func (v *DeployListView) Init() tea.Cmd {
	return v.list.Init()
}

func (v *DeployListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := v.list.Update(msg)
	return v, cmd
}

func (v *DeployListView) View() string {
	return v.list.View()
}

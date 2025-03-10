package views

import (
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/deploy"
	"github.com/render-oss/cli/pkg/tui"
)

type DeployListInput struct {
	ServiceID string `cli:"arg:0"`
}

func (in DeployListInput) Validate(interactive bool) error {
	if !interactive {
		return errors.New("service id must be specified when output is not interactive")
	}
	return nil
}

func LoadDeployList(ctx context.Context, input DeployListInput, cur client.Cursor) (client.Cursor, []*client.Deploy, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return "", nil, fmt.Errorf("failed to create client: %w", err)
	}

	pageSize := 20
	params := &client.ListDeploysParams{Limit: &pageSize}
	if cur != "" {
		params.Cursor = &cur
	}

	resp, err := c.ListDeploysWithResponse(ctx, input.ServiceID, params)
	if err != nil {
		return "", nil, fmt.Errorf("failed to list deploys: %w", err)
	}

	if resp.JSON200 == nil {
		return "", nil, fmt.Errorf("unexpected response: %s", resp.Status())
	}

	respOK := *resp.JSON200
	deploys := make([]*client.Deploy, len(respOK))
	for i, d := range respOK {
		deploys[i] = d.Deploy
	}

	if len(deploys) < pageSize {
		return "", deploys, nil
	}

	return *respOK[len(respOK)-1].Cursor, deploys, nil
}

type DeployListView struct {
	list *tui.List[*client.Deploy]
}

func NewDeployListView(ctx context.Context, input DeployListInput, generateCommands func(*client.Deploy) tea.Cmd) *DeployListView {
	onSelect := func(selectedItem tui.ListItem) tea.Cmd {
		selectedDeploy := selectedItem.(deploy.ListItem).Deploy()
		return generateCommands(selectedDeploy)
	}

	list := tui.NewList(
		"",
		command.PaginatedLoadCmd(ctx, LoadDeployList, input),
		func(d *client.Deploy) tui.ListItem {
			return deploy.NewListItem(d)
		},
		tui.WithOnSelect[*client.Deploy](onSelect),
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

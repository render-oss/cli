package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/deploy"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

var deployListCmd = &cobra.Command{
	Use:   "list [serviceID]",
	Short: "List deploys for a service",
	Args:  cobra.ExactArgs(1),
}

var InteractiveDeployList = command.Wrap(deployListCmd, loadDeployList, renderDeployList)

type DeployListInput struct {
	ServiceID string
}

func loadDeployList(ctx context.Context, input DeployListInput) ([]*client.Deploy, error) {
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

func renderDeployList(ctx context.Context, loadData func(DeployListInput) ([]*client.Deploy, error), input DeployListInput) (tea.Model, error) {
	loadFunc := func() ([]*client.Deploy, error) {
		return loadData(input)
	}

	list := tui.NewList(
		"Deploys",
		loadFunc,
		func(d *client.Deploy) tui.ListItem {
			return deploy.NewListItem(d)
		},
	)

	return list, nil
}

func init() {
	deployCmd.AddCommand(deployListCmd)

	deployListCmd.RunE = func(cmd *cobra.Command, args []string) error {
		serviceID := args[0]
		InteractiveDeployList(cmd.Context(), DeployListInput{ServiceID: serviceID})
		return nil
	}
}
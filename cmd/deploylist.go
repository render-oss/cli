package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/resource"
	"github.com/renderinc/cli/pkg/tui/views"
)

var deployListCmd = &cobra.Command{
	Use:   "list [serviceID]",
	Short: "List deploys for a service",
	Args:  cobra.ExactArgs(1),
}

var InteractiveDeployList = func(ctx context.Context, input views.DeployListInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, deployListCmd, breadcrumb, &input, views.NewDeployListView(ctx, input))
}

func init() {
	deployCmd.AddCommand(deployListCmd)

	deployListCmd.RunE = func(cmd *cobra.Command, args []string) error {
		serviceID := args[0]

		input := views.DeployListInput{ServiceID: serviceID}

		if nonInteractive, err := command.NonInteractive(
			cmd,
			func() (any, error) {
				return views.LoadDeployList(cmd.Context(), input)
			},
			nil,
		); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		r, err := resource.GetResource(cmd.Context(), serviceID)
		if err != nil {
			return err
		}

		InteractiveDeployList(cmd.Context(), input, "Deploys for "+resource.BreadcrumbForResource(r))
		return nil
	}
}

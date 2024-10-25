package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/tui/views"
	"github.com/spf13/cobra"
)

var deployListCmd = &cobra.Command{
	Use:   "list [serviceID]",
	Short: "List deploys for a service",
	Args:  cobra.ExactArgs(1),
}

var InteractiveDeployList = func(ctx context.Context, input views.DeployListInput) tea.Cmd {
	return command.AddToStackFunc(ctx, deployListCmd, &input, views.NewDeployListView(ctx, input))
}

func init() {
	deployCmd.AddCommand(deployListCmd)

	deployListCmd.RunE = func(cmd *cobra.Command, args []string) error {
		serviceID := args[0]

		input := views.DeployListInput{ServiceID: serviceID}

		if nonInteractive, err := command.NonInteractive(
			cmd.Context(),
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

		InteractiveDeployList(cmd.Context(), input)
		return nil
	}
}

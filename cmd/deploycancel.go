package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/tui/views"
)

var deployCancelCmd = &cobra.Command{
	Use:   "cancel [serviceID] [deployID]",
	Short: "Cancel a running deploy",
	Args:  cobra.ExactArgs(2),
}

var InteractiveDeployCancel = func(ctx context.Context, input views.DeployCancelInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, deployCancelCmd, breadcrumb, &input, views.NewDeployCancelView(ctx, input))
}

func init() {
	deployCmd.AddCommand(deployCancelCmd)
	deployCancelCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input views.DeployCancelInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		if nonInteractive, err := command.NonInteractive(
			cmd,
			func() (any, error) {
				return views.CancelDeploy(cmd.Context(), input)
			},
			func() (string, error) {
				return views.RequireConfirmationForCancelDeploy(cmd.Context(), input)
			},
		); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		InteractiveDeployCancel(cmd.Context(), input, "Cancel deploy "+input.DeployID)
		return nil
	}
}

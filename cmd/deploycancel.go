package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/views"
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

		nonInteractive, err := command.NonInteractiveWithConfirm(
			cmd,
			cancelDeploy(cmd.Context(), input),
			text.FormatString,
			confirmDeploy(cmd.Context(), input),
		)

		if err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		InteractiveDeployCancel(cmd.Context(), input, "Cancel deploy "+input.DeployID)
		return nil
	}
}

func cancelDeploy(ctx context.Context, input views.DeployCancelInput) func() (string, error) {
	return func() (string, error) {
		return views.CancelDeploy(ctx, input)
	}
}

func confirmDeploy(ctx context.Context, input views.DeployCancelInput) func() (string, error) {
	return func() (string, error) {
		return views.RequireConfirmationForCancelDeploy(ctx, input)
	}
}

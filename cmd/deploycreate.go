package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/tui/views"
	"github.com/renderinc/render-cli/pkg/types"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploys",
	Short: "Manage deployments",
}

var deployCreateCmd = &cobra.Command{
	Use:   "create [serviceID]",
	Short: "Deploy a service and tail logs",
	Args:  cobra.MaximumNArgs(1),
}

var InteractiveDeployCreate = func(ctx context.Context, input types.DeployInput) tea.Cmd {
	return command.AddToStackFunc(ctx, deployCreateCmd, &input, views.NewDeployCreateView(ctx, input, func(d *client.Deploy) tea.Cmd {
		return InteractiveLogs(ctx, views.LogInput{
			ResourceIDs: []string{input.ServiceID},
			Tail:        true,
		})
	}))
}

func init() {
	deployCreateCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input types.DeployInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}

		if nonInteractive, err := command.NonInteractive(
			cmd.Context(),
			cmd,
			func() (any, error) {
				return views.CreateDeploy(cmd.Context(), input)
			},
			views.DeployCreateConfirm(cmd.Context(), input),
		); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		InteractiveDeployCreate(cmd.Context(), input)
		return nil
	}

	deployCreateCmd.Flags().Bool("clear-cache", false, "Clear build cache before deploying")
	deployCreateCmd.Flags().String("commit", "", "The commit ID to deploy")
	deployCreateCmd.Flags().String("image", "", "The Docker image URL to deploy")

	deployCmd.AddCommand(deployCreateCmd)
	rootCmd.AddCommand(deployCmd)
}

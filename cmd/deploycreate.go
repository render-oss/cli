package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/resource"
	"github.com/renderinc/cli/pkg/text"
	"github.com/renderinc/cli/pkg/tui/views"
	"github.com/renderinc/cli/pkg/types"
)

var deployCmd = &cobra.Command{
	Use:     "deploys",
	Short:   "Manage service deploys",
	GroupID: GroupCore.ID,
}

var deployCreateCmd = &cobra.Command{
	Use:   "create [serviceID]",
	Short: "Trigger a service deploy and tail logs",
	Args:  cobra.MaximumNArgs(1),
}

var InteractiveDeployCreate = func(ctx context.Context, input types.DeployInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(
		ctx,
		deployCreateCmd,
		breadcrumb,
		&input,
		views.NewDeployCreateView(ctx, input, func(d *client.Deploy) tea.Cmd {
			return TailResourceLogs(ctx, input.ServiceID)
		}))
}

func init() {
	deployCreateCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input types.DeployInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}

		if nonInteractive, err := command.NonInteractiveWithConfirm(cmd, func() (*client.Deploy, error) {
			return views.CreateDeploy(cmd.Context(), input)
		}, func(deploy *client.Deploy) string {
			return text.FormatStringF("Created deploy %s for service %s", deploy.Id, input.ServiceID)
		}, views.DeployCreateConfirm(cmd.Context(), input)); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		service, err := resource.GetResource(cmd.Context(), input.ServiceID)
		if err != nil {
			return err
		}

		InteractiveDeployCreate(cmd.Context(), input, "Create Deploy for "+resource.BreadcrumbForResource(service))
		return nil
	}

	deployCreateCmd.Flags().Bool("clear-cache", false, "Clear build cache before deploying")
	deployCreateCmd.Flags().String("commit", "", "The commit ID to deploy")
	deployCreateCmd.Flags().String("image", "", "The Docker image URL to deploy")

	deployCmd.AddCommand(deployCreateCmd)
	rootCmd.AddCommand(deployCmd)
}

package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/deploy"
	"github.com/renderinc/render-cli/pkg/pointers"
	"github.com/renderinc/render-cli/pkg/service"
	"github.com/renderinc/render-cli/pkg/tui"
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

var InteractiveDeployCreate = command.Wrap(deployCmd, createDeploy, renderCreateDeploy, &command.WrapOptions[types.DeployInput]{
	RequireConfirm: command.RequireConfirm[types.DeployInput]{
		Confirm: true,
		MessageFunc: func(ctx context.Context, args types.DeployInput) (string, error) {
			c, err := client.NewDefaultClient()
			if err != nil {
				return "", fmt.Errorf("failed to create client: %w", err)
			}

			serviceRepo := service.NewRepo(c)
			srv, err := serviceRepo.GetService(ctx, args.ServiceID)
			if err != nil {
				return "", fmt.Errorf("failed to get service: %w", err)
			}
			return fmt.Sprintf("Are you sure you want to deploy %s?", srv.Name), nil
		},
	},
})

func createDeploy(ctx context.Context, input types.DeployInput) (*client.Deploy, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	deployRepo := deploy.NewRepo(c)

	if input.CommitID != nil && *input.CommitID == "" {
		input.CommitID = nil
	}

	if input.ImageURL != nil && *input.ImageURL == "" {
		input.ImageURL = nil
	}

	d, err := deployRepo.TriggerDeploy(ctx, input.ServiceID, deploy.TriggerDeployInput{
		ClearCache: &input.ClearCache,
		CommitId:   input.CommitID,
		ImageUrl:   input.ImageURL,
	})
	if err != nil {
		return nil, err
	}

	return d, nil
}

func renderCreateDeploy(ctx context.Context, loadData func(types.DeployInput) tui.TypedCmd[*client.Deploy], input types.DeployInput) (tea.Model, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	serviceRepo := service.NewRepo(c)
	svc, err := serviceRepo.GetService(ctx, input.ServiceID)
	if err != nil {
		return nil, err
	}

	var inputs []huh.Field
	if svc.ImagePath != nil {
		if input.ImageURL == nil {
			input.ImageURL = pointers.From("")
		}

		inputs = append(inputs, huh.NewInput().
			Title("Image URL").
			Placeholder("Enter Docker image URL (optional)").
			Value(input.ImageURL))
	} else {
		if input.CommitID == nil {
			input.CommitID = pointers.From("")
		}

		inputs = append(inputs, huh.NewInput().
			Title("Commit ID").
			Placeholder("Enter commit ID (optional)").
			Value(input.CommitID))
	}

	deployForm := huh.NewForm(huh.NewGroup(inputs...))
	logAction := func(_ *client.Deploy) tea.Cmd {
		return InteractiveLogs(ctx, LogInput{
			ResourceIDs: []string{input.ServiceID},
			Tail:        true,
		})
	}

	action := tui.NewFormAction(
		logAction,
		loadData(input),
	)

	return tui.NewFormWithAction(action, deployForm), nil
}

func init() {
	deployCreateCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input types.DeployInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
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

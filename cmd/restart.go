package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart [resourceID]",
	Short: "Restart a service",
	Args:  cobra.ExactArgs(1),
}

var InteractiveRestart = command.Wrap(
	restartCmd,
	restartResource,
	renderRestart,
	&command.WrapOptions[RestartInput]{
		RequireConfirm: command.RequireConfirm[RestartInput]{
			Confirm: true,
			MessageFunc: func(ctx context.Context, args RestartInput) (string, error) {
				resourceService, err := newResourceService()
				if err != nil {
					return "", fmt.Errorf("failed to create resource service: %w", err)
				}

				res, err := resourceService.GetResource(ctx, args.ResourceID)

				return fmt.Sprintf("Are you sure you want to restart resource %s?", res.Name()), nil
			},
		},
	},
)

type RestartInput struct {
	ResourceID string `cli:"arg:0"`
}

func restartResource(ctx context.Context, input RestartInput) (string, error) {
	resourceService, err := newResourceService()
	if err != nil {
		return "", fmt.Errorf("failed to create resource service: %w", err)
	}

	err = resourceService.RestartResource(ctx, input.ResourceID)
	if err != nil {
		return "", fmt.Errorf("failed to restart resource: %w", err)
	}

	return fmt.Sprintf("%s restarted successfully", input.ResourceID), nil
}

func renderRestart(_ context.Context, loadData func(RestartInput) (string, error), in RestartInput) (tea.Model, error) {
	return tui.NewSimpleModel(func() (string, error) {
		return loadData(in)
	}), nil
}

func init() {
	restartCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input RestartInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		InteractiveRestart(cmd.Context(), input)
		return nil
	}

	rootCmd.AddCommand(restartCmd)
}

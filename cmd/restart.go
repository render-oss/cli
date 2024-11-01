package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/resource"
	"github.com/renderinc/render-cli/pkg/tui/views"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart [resourceID]",
	Short: "Restart a service",
	Args:  cobra.ExactArgs(1),
}

var InteractiveRestart = func(ctx context.Context, input views.RestartInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, restartCmd, breadcrumb, &input, views.NewRestartView(ctx, input))
}

func init() {
	restartCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input views.RestartInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		if nonInteractive, err := command.NonInteractive(
			cmd.Context(),
			cmd,
			func() (any, error) {
				return views.RestartResource(cmd.Context(), input)
			},
			nil,
		); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		r, err := resource.GetResource(cmd.Context(), input.ResourceID)
		if err != nil {
			return err
		}
		InteractiveRestart(cmd.Context(), input, "Restart " + resource.BreadcrumbForResource(r))
		return nil
	}

	rootCmd.AddCommand(restartCmd)
}

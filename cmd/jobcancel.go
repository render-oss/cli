package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/tui/views"
	"github.com/spf13/cobra"
)

var jobCancelCmd = &cobra.Command{
	Use:   "cancel [serviceID] [jobID]",
	Short: "Cancel a running job",
	Args:  cobra.ExactArgs(2),
}

var InteractiveJobCancel = func(ctx context.Context, input views.JobCancelInput) tea.Cmd {
	return command.AddToStackFunc(ctx, jobCancelCmd, &input, views.NewJobCancelView(ctx, input))
}

func init() {
	jobCancelCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input views.JobCancelInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		if nonInteractive, err := command.NonInteractive(
			cmd.Context(),
			cmd,
			func() (any, error) {
				return views.CancelJob(cmd.Context(), input)
			},
			func() (string, error) {
				return views.RequireConfirmationForCancelJob(cmd.Context(), input)
			},
		); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		InteractiveJobCancel(cmd.Context(), input)
		return nil
	}
}

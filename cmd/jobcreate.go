package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	clientjob "github.com/renderinc/cli/pkg/client/jobs"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/resource"
	"github.com/renderinc/cli/pkg/tui/views"
)

var jobCreateCmd = &cobra.Command{
	Use:   "create [serviceID]",
	Short: "Create a new job for a service",
	Args:  cobra.ExactArgs(1),
}

var InteractiveJobCreate = func(ctx context.Context, input *views.JobCreateInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(
		ctx,
		jobCreateCmd,
		breadcrumb,
		input,
		views.NewJobCreateView(ctx, input, jobCreateCmd, func(j *clientjob.Job) tea.Cmd {
			return InteractiveLogs(ctx, views.LogInput{
				ResourceIDs: []string{j.Id},
				Tail:        true,
			}, "Logs")
		}),
	)
}

func init() {
	jobCreateCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input views.JobCreateInput

		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}

		if nonInteractive, err := command.NonInteractive(
			cmd,
			func() (any, error) {
				return views.CreateJob(cmd.Context(), input)
			},
			nil,
		); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		r, err := resource.GetResource(cmd.Context(), input.ServiceID)
		if err != nil {
			return err
		}

		InteractiveJobCreate(cmd.Context(), &input, "Create Job "+resource.BreadcrumbForResource(r))
		return nil
	}

	jobCreateCmd.Flags().String("start-command", "", "The command to run for the job")
	jobCreateCmd.Flags().String("plan-id", "", "The plan ID for the job (optional)")
}

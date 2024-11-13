package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	clientjob "github.com/renderinc/cli/pkg/client/jobs"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/job"
	"github.com/renderinc/cli/pkg/pointers"
	"github.com/renderinc/cli/pkg/resource"
	"github.com/renderinc/cli/pkg/tui/views"
)

var jobListCmd = &cobra.Command{
	Use:   "list [serviceID]",
	Short: "List jobs for a service",
	Args:  cobra.ExactArgs(1),
}

var InteractiveJobList = func(ctx context.Context, input views.JobListInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, jobListCmd, breadcrumb, &views.ProjectInput{}, views.NewJobListView(ctx,
		&input,
		func(j *clientjob.Job) tea.Cmd {
			return InteractivePalette(ctx, commandsForJob(j), j.Id)
		},
	))
}

func commandsForJob(j *clientjob.Job) []views.PaletteCommand {
	var startTime *string
	if j.StartedAt != nil {
		startTime = pointers.From(j.StartedAt.String())
	}

	var endTime *string
	if j.FinishedAt != nil {
		endTime = pointers.From(j.FinishedAt.String())
	}

	commands := []views.PaletteCommand{
		{
			Name:        "logs",
			Description: "View job logs",
			Action: func(ctx context.Context, args []string) tea.Cmd {
				return InteractiveLogs(
					ctx,
					views.LogInput{
						ResourceIDs: []string{j.Id},
						StartTime:   startTime,
						EndTime:     endTime,
					},
					"Logs",
				)
			},
		},
		{
			Name:        "rerun",
			Description: "Create new job with same inputs",
			Action: func(ctx context.Context, args []string) tea.Cmd {
				return InteractiveJobCreate(ctx, &views.JobCreateInput{
					ServiceID:    j.ServiceId,
					StartCommand: &j.StartCommand,
					PlanID:       &j.PlanId,
				},
					"Rerun",
				)
			},
		},
	}

	if job.IsCancellable(j.Status) {
		commands = append(commands, views.PaletteCommand{
			Name:        "cancel",
			Description: "Cancel the job",
			Action: func(ctx context.Context, args []string) tea.Cmd {
				return InteractiveJobCancel(
					ctx,
					views.JobCancelInput{ServiceID: j.ServiceId, JobID: j.Id},
					"Cancel job",
				)
			},
		})
	}

	return commands
}

func init() {
	jobListCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input views.JobListInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}

		if nonInteractive, err := command.NonInteractive(
			cmd,
			func() (any, error) {
				return views.LoadJobListData(cmd.Context(), input)
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

		InteractiveJobList(cmd.Context(), input, "Jobs for "+resource.BreadcrumbForResource(r))
		return nil
	}
}

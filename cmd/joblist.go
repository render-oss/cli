package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	clientjob "github.com/render-oss/cli/pkg/client/jobs"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/job"
	"github.com/render-oss/cli/pkg/resource"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/flows"
	"github.com/render-oss/cli/pkg/tui/views"
)

var jobListCmd = &cobra.Command{
	Use:   "list [serviceID]",
	Short: "List jobs for a service",
	Args:  cobra.MaximumNArgs(1),
}

var InteractiveJobList = func(ctx context.Context, input views.JobListInput, breadcrumb string) tea.Cmd {
	deps := dependencies.GetFromContext(ctx)
	return command.AddToStackFunc(ctx, jobListCmd, breadcrumb, &views.ProjectInput{}, views.NewJobListView(ctx,
		&input,
		func(j *clientjob.Job) tea.Cmd {
			return InteractivePalette(ctx, commandsForJob(deps, j), j.Id)
		},
	))
}

func interactiveJobList(cmd *cobra.Command, input views.JobListInput) tea.Cmd {
	ctx := cmd.Context()
	if input.ServiceID == "" {
		return command.AddToStackFunc(
			ctx,
			cmd,
			"Jobs",
			&input,
			views.NewServiceList(ctx, views.ServiceInput{
				Types: []client.ServiceType{
					client.WebService, client.BackgroundWorker, client.PrivateService, client.CronJob,
				},
			}, func(ctx context.Context, r resource.Resource) tea.Cmd {
				input.ServiceID = r.ID()
				return InteractiveJobList(ctx, input, resource.BreadcrumbForResource(r))
			}),
		)
	}

	service, err := resource.GetResource(ctx, input.ServiceID)
	if err != nil {
		command.Fatal(cmd, err)
	}

	return InteractiveJobList(ctx, input, "Jobs for "+resource.BreadcrumbForResource(service))
}

func commandsForJob(deps *dependencies.Dependencies, j *clientjob.Job) []views.PaletteCommand {
	var startTime *command.TimeOrRelative
	if j.StartedAt != nil {
		startTime = &command.TimeOrRelative{T: j.StartedAt}
	}

	var endTime *command.TimeOrRelative
	if j.FinishedAt != nil {
		endTime = &command.TimeOrRelative{T: j.FinishedAt}
	}

	commands := []views.PaletteCommand{
		{
			Name:        "logs",
			Description: "View job logs",
			Action: func(ctx context.Context, args []string) tea.Cmd {
				return flows.NewLogFlow(deps).LogsFlow(
					ctx,
					views.LogInput{
						ResourceIDs: []string{j.Id},
						StartTime:   startTime,
						EndTime:     endTime,
					},
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

		if nonInteractive, err := command.NonInteractive(cmd, func() ([]*clientjob.Job, error) {
			_, jobs, err := views.LoadJobListData(cmd.Context(), input, "")
			return jobs, err
		}, text.JobTable); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		interactiveJobList(cmd, input)
		return nil
	}
}

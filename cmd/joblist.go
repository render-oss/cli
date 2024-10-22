package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	clientjob "github.com/renderinc/render-cli/pkg/client/jobs"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/job"
	"github.com/renderinc/render-cli/pkg/pointers"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

var jobListCmd = &cobra.Command{
	Use:   "list [serviceID]",
	Short: "List jobs for a service",
	Args:  cobra.ExactArgs(1),
}

var InteractiveJobList = command.Wrap(jobListCmd, loadJobListData, renderJobList)

type JobListInput struct {
	ServiceID string `cli:"arg:0"`
}

func loadJobListData(ctx context.Context, input JobListInput) ([]*clientjob.Job, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	jobRepo := job.NewRepo(c)

	return jobRepo.ListJobs(ctx, job.ListJobsInput{
		ServiceID: input.ServiceID,
	})
}

func renderJobList(ctx context.Context, loadData func(JobListInput) ([]*clientjob.Job, error), input JobListInput) (tea.Model, error) {
	loadFunc := func() ([]*clientjob.Job, error) {
		return loadData(input)
	}

	list := tui.NewList(
		"Jobs",
		loadFunc,
		func(j *clientjob.Job) tui.ListItem {
			return job.NewListItem(j)
		},
		tui.WithOnSelect[*clientjob.Job](func(selectedItem tui.ListItem) tea.Cmd {
			selectedJob := selectedItem.(job.ListItem).Job()
			return selectJob(ctx, input.ServiceID)(selectedJob)
		}),
	)

	return list, nil
}

func selectJob(ctx context.Context, serviceID string) func(*clientjob.Job) tea.Cmd {
	return func(j *clientjob.Job) tea.Cmd {
		var startTime *string
		if j.StartedAt != nil {
			startTime = pointers.From(j.StartedAt.String())
		}

		var endTime *string
		if j.FinishedAt != nil {
			endTime = pointers.From(j.FinishedAt.String())
		}

		commands := []PaletteCommand{
			{
				Name:        "logs",
				Description: "View job logs",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return InteractiveLogs(ctx, LogInput{
						ResourceIDs: []string{j.Id},
						StartTime:   startTime,
						EndTime:     endTime,
					})
				},
			},
			{
				Name:        "rerun",
				Description: "Create new job with same inputs",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return InteractiveJobCreate(ctx, JobCreateInput{
						ServiceID:    serviceID,
						StartCommand: &j.StartCommand,
						PlanID:       &j.PlanId,
					})
				},
			},
		}

		if job.IsCancellable(j.Status) {
			commands = append(commands, PaletteCommand{
				Name:        "cancel",
				Description: "Cancel the job",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return InteractiveJobCancel(ctx, JobCancelInput{ServiceID: serviceID, JobID: j.Id})
				},
			})
		}

		return InteractiveCommandPalette(ctx, PaletteCommandInput{
			Commands: commands,
		})
	}
}

func init() {
	jobListCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input JobListInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}
		InteractiveJobList(cmd.Context(), input)
		return nil
	}
}

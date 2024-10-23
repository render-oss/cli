package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	clientjob "github.com/renderinc/render-cli/pkg/client/jobs"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/job"
	"github.com/renderinc/render-cli/pkg/service"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

var jobCancelCmd = &cobra.Command{
	Use:   "cancel [serviceID] [jobID]",
	Short: "Cancel a running job",
	Args:  cobra.ExactArgs(2),
}

var InteractiveJobCancel = command.Wrap(jobCancelCmd, cancelJob, renderJobCancel, &command.WrapOptions[JobCancelInput]{
	RequireConfirm: command.RequireConfirm[JobCancelInput]{
		Confirm: true,
		MessageFunc: func(ctx context.Context, args JobCancelInput) (string, error) {
			c, err := client.NewDefaultClient()
			if err != nil {
				return "", fmt.Errorf("failed to create client: %w", err)
			}

			serviceRepo := service.NewRepo(c)
			srv, err := serviceRepo.GetService(ctx, args.ServiceID)
			if err != nil {
				return "", fmt.Errorf("failed to get service: %w", err)
			}
			return fmt.Sprintf("Are you sure you want to cancel job %s for Service %s?", args.JobID, srv.Name), nil
		},
	},
})

type JobCancelInput struct {
	ServiceID string `cli:"arg:0"`
	JobID     string `cli:"arg:1"`
}

func cancelJob(ctx context.Context, input JobCancelInput) (*clientjob.Job, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	jobRepo := job.NewRepo(c)

	return jobRepo.CancelJob(ctx, input.ServiceID, input.JobID)
}

func renderJobCancel(ctx context.Context, cancelJobFunc func(JobCancelInput) (*clientjob.Job, error), input JobCancelInput) (tea.Model, error) {
	loadFunc := func() (string, error) {
		j, err := cancelJobFunc(input)
		if err != nil {
			return "", fmt.Errorf("failed to cancel job: %w", err)
		}

		return fmt.Sprintf("Job %s successfuly cancelled", j.Id), nil
	}

	return tui.NewSimpleModel(loadFunc), nil
}

func init() {
	jobCancelCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input JobCancelInput

		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		InteractiveJobCancel(cmd.Context(), input)
		return nil
	}
}

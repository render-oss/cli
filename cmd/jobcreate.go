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

var jobCreateCmd = &cobra.Command{
	Use:   "create [serviceID]",
	Short: "Create a new job for a service",
	Args:  cobra.ExactArgs(1),
}

var InteractiveJobCreate = command.Wrap(jobCreateCmd, createJob, renderJobCreate)

type JobCreateInput struct {
	ServiceID    string `cli:"arg:0"`
	StartCommand *string `cli:"start-command"`
	PlanID       *string	`cli:"plan-id"`
}

func createJob(ctx context.Context, input JobCreateInput) (*clientjob.Job, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	jobRepo := job.NewRepo(c)

	return jobRepo.CreateJob(ctx, job.CreateJobInput{
		ServiceID:    input.ServiceID,
		StartCommand: pointers.ValueOrDefault(input.StartCommand, ""),
		PlanID:       pointers.ValueOrDefault(input.PlanID, ""),
	})
}

func renderJobCreate(ctx context.Context, createJobFunc func(JobCreateInput) (*clientjob.Job, error), in JobCreateInput) (tea.Model, error) {
	form, result := command.HuhForm(jobCreateCmd, &in)
	var jobCreateInput JobCreateInput
	err := command.StructFromFormValues(result, &jobCreateInput)
	if err != nil {
		return nil, err
	}

	logData := func(in LogInput) (*LogResult, error) {
		return loadLogData(ctx, in)
	}

	logModelFunc := func(resourceID string) (tea.Model, error) {
		model, err := renderLogs(ctx, logData, LogInput{
			ResourceIDs: []string{resourceID},
			Tail:        true,
		})
		if err != nil {
			return nil, err
		}
		return model, nil
	}

	onSubmit := func() tea.Cmd {
		return func() tea.Msg {
			createdJob, err := createJobFunc(jobCreateInput)
			if err != nil {
				return tui.ErrorMsg{Err: fmt.Errorf("failed to create job: %w", err)}
			}

			return tui.SubmittedMsg{ID: createdJob.Id}
		}
	}

	action := tui.NewFormAction(
		logModelFunc,
		onSubmit,
	)

	return tui.NewFormWithAction(action, form), nil
}

func init() {
	jobCreateCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input JobCreateInput

		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}

		InteractiveJobCreate(cmd.Context(), input)
		return nil
	}

	jobCreateCmd.Flags().String("start-command", "", "The command to run for the job")
	jobCreateCmd.Flags().String("plan-id", "", "The plan ID for the job (optional)")
}

package views

import (
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	clientjob "github.com/render-oss/cli/pkg/client/jobs"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/job"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/tui"
)

type JobCreateInput struct {
	ServiceID    string  `cli:"arg:0"`
	StartCommand *string `cli:"start-command"`
	PlanID       *string `cli:"plan-id"`
}

func (j JobCreateInput) Validate(interactive bool) error {
	if !interactive {
		return errors.New("service id must be specified when output is not interactive")
	}
	return nil
}

func CreateJob(ctx context.Context, input JobCreateInput) (*clientjob.Job, error) {
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

type JobCreateView struct {
	formAction *tui.FormWithAction[*clientjob.Job]
}

func NewJobCreateView(
	ctx context.Context,
	input *JobCreateInput,
	cobraCmd *cobra.Command,
	createJob func(ctx context.Context, input JobCreateInput) (*clientjob.Job, error),
	action func(j *clientjob.Job) tea.Cmd,
) *JobCreateView {
	fields, values := command.HuhFormFields(cobraCmd, input)

	return &JobCreateView{
		formAction: tui.NewFormWithAction(
			tui.NewFormAction(
				action,
				func() tea.Msg {
					var createJobInput JobCreateInput
					err := command.StructFromFormValues(values, &createJobInput)
					if err != nil {
						return tui.ErrorMsg{Err: err}
					}
					return command.LoadCmd(ctx, createJob, createJobInput)()
				},
			),
			huh.NewForm(huh.NewGroup(fields...)),
		),
	}
}

func (v *JobCreateView) Init() tea.Cmd {
	return v.formAction.Init()
}

func (v *JobCreateView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v.formAction.Update(msg)
}

func (v *JobCreateView) View() string {
	return v.formAction.View()
}

package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/cli/pkg/client"
	clientjob "github.com/renderinc/cli/pkg/client/jobs"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/job"
	"github.com/renderinc/cli/pkg/pointers"
	"github.com/renderinc/cli/pkg/tui"
	"github.com/spf13/cobra"
)

type JobCreateInput struct {
	ServiceID    string  `cli:"arg:0"`
	StartCommand *string `cli:"start-command"`
	PlanID       *string `cli:"plan-id"`
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

func NewJobCreateView(ctx context.Context, input *JobCreateInput, cobraCmd *cobra.Command, action func(j *clientjob.Job) tea.Cmd) *JobCreateView {
	form, _ := command.HuhForm(cobraCmd, input)

	return &JobCreateView{
		formAction: tui.NewFormWithAction(
			tui.NewFormAction(
				action,
				command.LoadCmd(ctx, CreateJob, *input),
			),
			form,
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

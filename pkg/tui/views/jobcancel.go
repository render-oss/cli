// pkg/tui/views/jobcancelview.go
package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/job"
	"github.com/renderinc/cli/pkg/service"
	"github.com/renderinc/cli/pkg/tui"
)

type JobCancelInput struct {
	ServiceID string `cli:"arg:0"`
	JobID     string `cli:"arg:1"`
}

type JobCancelView struct {
	model *tui.SimpleModel
}

func CancelJob(ctx context.Context, input JobCancelInput) (string, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	jobRepo := job.NewRepo(c)

	_, err = jobRepo.CancelJob(ctx, input.ServiceID, input.JobID)
	if err != nil {
		return "", fmt.Errorf("failed to cancel job: %w", err)
	}
	return fmt.Sprintf("Job %s successfully cancelled", input.JobID), nil
}

func RequireConfirmationForCancelJob(ctx context.Context, input JobCancelInput) (string, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	serviceRepo := service.NewRepo(c)
	srv, err := serviceRepo.GetService(ctx, input.ServiceID)
	if err != nil {
		return "", fmt.Errorf("failed to get service: %w", err)
	}
	return fmt.Sprintf("Are you sure you want to cancel job %s for Service %s?", input.JobID, srv.Name), nil
}

func NewJobCancelView(ctx context.Context, input JobCancelInput) *JobCancelView {
	// Create simple model that will show the result
	model := tui.NewSimpleModel(command.WrapInConfirm(
		command.LoadCmd(ctx, CancelJob, input),
		func() (string, error) { return RequireConfirmationForCancelJob(ctx, input) },
	))

	return &JobCancelView{
		model: model,
	}
}

func (v *JobCancelView) Init() tea.Cmd {
	return v.model.Init()
}

func (v *JobCancelView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v.model.Update(msg)
}

func (v *JobCancelView) View() string {
	return v.model.View()
}

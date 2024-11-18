package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/deploy"
	"github.com/renderinc/cli/pkg/service"
	"github.com/renderinc/cli/pkg/tui"
)

type DeployCancelInput struct {
	ServiceID string `cli:"arg:0"`
	DeployID     string `cli:"arg:1"`
}

type DeployCancelView struct {
	model *tui.SimpleModel
}

func CancelDeploy(ctx context.Context, input DeployCancelInput) (string, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	deployRepo := deploy.NewRepo(c)

	_, err = deployRepo.CancelDeploy(ctx, input.ServiceID, input.DeployID)
	if err != nil {
		return "", fmt.Errorf("failed to cancel deploy: %w", err)
	}
	return fmt.Sprintf("Deploy %s successfully cancelled", input.DeployID), nil
}

func RequireConfirmationForCancelDeploy(ctx context.Context, input DeployCancelInput) (string, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	serviceRepo := service.NewRepo(c)
	srv, err := serviceRepo.GetService(ctx, input.ServiceID)
	if err != nil {
		return "", fmt.Errorf("failed to get service: %w", err)
	}
	return fmt.Sprintf("Are you sure you want to cancel deploy %s for Service %s?", input.DeployID, srv.Name), nil
}

func NewDeployCancelView(ctx context.Context, input DeployCancelInput) *DeployCancelView {
	model := tui.NewSimpleModel(command.WrapInConfirm(
		command.LoadCmd(ctx, CancelDeploy, input),
		func() (string, error) { return RequireConfirmationForCancelDeploy(ctx, input) },
	))

	return &DeployCancelView{
		model: model,
	}
}

func (v *DeployCancelView) Init() tea.Cmd {
	return v.model.Init()
}

func (v *DeployCancelView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v.model.Update(msg)
}

func (v *DeployCancelView) View() string {
	return v.model.View()
}

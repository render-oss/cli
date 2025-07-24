package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/types"
)

type ServiceCreateView struct {
	model *tui.SimpleModel
}

func CreateService(ctx context.Context, input types.ServiceCreateInput) (string, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	serviceRepo := service.NewRepo(c)

	// Get workspace ID
	workspace, err := config.WorkspaceID()
	if err != nil {
		return "", err
	}
	if workspace == "" {
		return "", fmt.Errorf("workspace is required")
	}

	// Build request
	req := client.ServicePOST{
		Name:    input.Name,
		OwnerId: workspace,
		Type:    input.Type,
	}

	// Set common fields
	if input.Repo != "" {
		req.Repo = pointers.From(input.Repo)
	}
	if input.Branch != "" {
		req.Branch = pointers.From(input.Branch)
	}
	if input.RootDir != "" {
		req.RootDir = pointers.From(input.RootDir)
	}

	// Create the service
	svc, err := serviceRepo.CreateService(ctx, req)
	if err != nil {
		return "", err
	}

	message := fmt.Sprintf("Service created: %s (%s)", svc.Name, svc.Id)
	return message, nil
}

func NewServiceCreateView(ctx context.Context, input types.ServiceCreateInput, onCreate func(*client.Service) tea.Cmd) *ServiceCreateView {
	model := tui.NewSimpleModel(command.LoadCmd(ctx, CreateService, input))

	return &ServiceCreateView{
		model: model,
	}
}

func (v *ServiceCreateView) Init() tea.Cmd {
	return v.model.Init()
}

func (v *ServiceCreateView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v.model.Update(msg)
}

func (v *ServiceCreateView) View() string {
	return v.model.View()
}
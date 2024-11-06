package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/postgres"
	"github.com/renderinc/render-cli/pkg/project"
	"github.com/renderinc/render-cli/pkg/redis"
	"github.com/renderinc/render-cli/pkg/resource"
	"github.com/renderinc/render-cli/pkg/service"
	"github.com/renderinc/render-cli/pkg/tui"
)

type RestartInput struct {
	ResourceID string `cli:"arg:0"`
}

func RestartResource(ctx context.Context, input RestartInput) (string, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return "", err
	}

	serviceRepo := service.NewRepo(c)
	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)
	postgresRepo := postgres.NewRepo(c)
	redisRepo := redis.NewRepo(c)

	serviceService := service.NewService(serviceRepo, environmentRepo, projectRepo)
	postgresService := postgres.NewService(postgresRepo, environmentRepo, projectRepo)
	redisService := redis.NewService(redisRepo, environmentRepo, projectRepo)

	resourceService := resource.NewResourceService(
		serviceService,
		postgresService,
		redisService,
		environmentRepo,
		projectRepo,
	)

	if err != nil {
		return "", fmt.Errorf("failed to create resource service: %w", err)
	}

	err = resourceService.RestartResource(ctx, input.ResourceID)
	if err != nil {
		return "", fmt.Errorf("failed to restart resource: %w", err)
	}

	return fmt.Sprintf("%s restarted successfully", input.ResourceID), nil
}

type RestartView struct {
	model *tui.SimpleModel
}

func NewRestartView(ctx context.Context, input RestartInput) *RestartView {
	return &RestartView{
		model: tui.NewSimpleModel(command.WrapInConfirm(
			command.LoadCmd(ctx, RestartResource, input),
			func() (string, error) {
				res, err := resource.GetResource(ctx, input.ResourceID)
				if err != nil {
					return "", fmt.Errorf("failed to get resource: %w", err)
				}

				return fmt.Sprintf("Are you sure you want to restart resource %s?", res.Name()), nil
			},
		)),
	}
}

func (v *RestartView) Init() tea.Cmd {
	return v.model.Init()
}

func (v *RestartView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := v.model.Update(msg)
	return v, cmd
}

func (v *RestartView) View() string {
	return v.model.View()
}

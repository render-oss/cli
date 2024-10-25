package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/postgres"
	"github.com/renderinc/render-cli/pkg/project"
	"github.com/renderinc/render-cli/pkg/resource"
	resourcetui "github.com/renderinc/render-cli/pkg/resource/tui"
	"github.com/renderinc/render-cli/pkg/service"
	"github.com/renderinc/render-cli/pkg/tui"
)

type ListResourceInput struct {
	EnvironmentID string `cli:"environment"`
}

func (l ListResourceInput) ToParams() resource.ResourceParams {
	return resource.ResourceParams{
		EnvironmentID: l.EnvironmentID,
	}
}

type ResourceView struct {
	table *tui.Table[resource.Resource]
}

func newResourceService(c *client.ClientWithResponses) *resource.Service {
	serviceRepo := service.NewRepo(c)
	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)
	postgresRepo := postgres.NewRepo(c)

	serviceService := service.NewService(serviceRepo, environmentRepo, projectRepo)
	postgresService := postgres.NewService(postgresRepo, environmentRepo, projectRepo)

	return resource.NewResourceService(
		serviceService,
		postgresService,
		environmentRepo,
		projectRepo,
	)
}

func LoadResourceData(ctx context.Context, in ListResourceInput) ([]resource.Resource, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	resourceService := newResourceService(c)

	return resourceService.ListResources(ctx, in.ToParams())
}

func NewResourceView(ctx context.Context, input ListResourceInput, onSelect func(r resource.Resource) tea.Cmd, opts ...tui.TableOption[resource.Resource]) *ResourceView {
	resourceView := &ResourceView{}

	onSelectWrapper := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		r, ok := rows[0].Data["resource"].(resource.Resource)
		if !ok {
			return nil
		}

		return onSelect(r)
	}

	resourceView.table = tui.NewTable(
		resourcetui.ColumnsForResources(),
		command.LoadCmd(ctx, LoadResourceData, input),
		resourcetui.RowForResource,
		onSelectWrapper,
		opts...,
	)

	return resourceView
}

func (v *ResourceView) Init() tea.Cmd {
	return v.table.Init()
}

func (v *ResourceView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := v.table.Update(msg)
	return v, cmd
}

func (v *ResourceView) View() string {
	return v.table.View()
}

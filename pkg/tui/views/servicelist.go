package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/resource"
	resourcetui "github.com/render-oss/cli/pkg/resource/tui"
	"github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/tui"
)

type ServiceList struct {
	table *tui.Table[*service.Model]
}

func NewServiceList(ctx context.Context, in ServiceInput, selectFunc OnSelectFuncT[resource.Resource], opts ...tui.TableOption[*service.Model]) *ServiceList {
	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		r, ok := rows[0].Data["resource"].(resource.Resource)
		if !ok {
			return nil
		}

		return selectFunc(ctx, r)
	}

	t := tui.NewTable(
		resourcetui.ColumnsForResources(),
		command.LoadCmd(ctx, listServices, in),
		func(s *service.Model) btable.Row {
			return resourcetui.RowForResource(s)
		},
		onSelect,
		opts...,
	)

	return &ServiceList{
		table: t,
	}
}

type ServiceInput struct {
	Project         *client.Project
	EnvironmentIDs  []string
	IncludePreviews bool
	Types           []client.ServiceType
}

func listServices(ctx context.Context, in ServiceInput) ([]*service.Model, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	serviceRepo := service.NewRepo(c)
	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)

	serviceService := service.NewService(serviceRepo, environmentRepo, projectRepo)

	listInput := &client.ListServicesParams{
		IncludePreviews: pointers.From(in.IncludePreviews),
		Limit:           pointers.From(100),
	}

	if len(in.Types) > 0 {
		listInput.Type = &in.Types
	}

	if len(in.EnvironmentIDs) > 0 {
		listInput.EnvironmentId = &in.EnvironmentIDs
	}

	return serviceService.ListServices(ctx, listInput)
}

func (pl *ServiceList) Init() tea.Cmd {
	return pl.table.Init()
}

func (pl *ServiceList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return pl.table.Update(msg)
}

func (pl *ServiceList) View() string {
	return pl.table.View()
}

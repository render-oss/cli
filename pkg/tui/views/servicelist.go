package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/pointers"
	"github.com/renderinc/render-cli/pkg/project"
	"github.com/renderinc/render-cli/pkg/resource"
	resourcetui "github.com/renderinc/render-cli/pkg/resource/tui"
	"github.com/renderinc/render-cli/pkg/service"
	"github.com/renderinc/render-cli/pkg/tui"
)

type ServiceList struct {
	table *tui.Table[*service.Model]
}

func NewServiceList(ctx context.Context, selectFunc OnSelectFuncT[resource.Resource]) *ServiceList {
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
		command.LoadCmd(ctx, listServices, ServiceInput{}),
		func(s *service.Model) btable.Row {
			return resourcetui.RowForResource(s)
		},
		onSelect,
	)

	return &ServiceList{
		table: t,
	}
}

type ServiceInput struct{}

func listServices(ctx context.Context, _ ServiceInput) ([]*service.Model, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	serviceRepo := service.NewRepo(c)
	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)

	serviceService := service.NewService(serviceRepo, environmentRepo, projectRepo)

	return serviceService.ListServices(ctx, &client.ListServicesParams{
		Type:  &[]client.ServiceType{client.WebService, client.PrivateService, client.BackgroundWorker},
		Limit: pointers.From(100),
	})
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

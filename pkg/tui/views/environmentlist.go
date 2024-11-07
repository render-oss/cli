package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/tui"
)

type EnvironmentInput struct {
	ProjectID string `cli:"arg:0"`
}

func (e EnvironmentInput) ToParams() *client.ListEnvironmentsParams {
	return &client.ListEnvironmentsParams{
		ProjectId: []string{e.ProjectID},
	}
}

func LoadEnvironments(ctx context.Context, in EnvironmentInput) ([]*client.Environment, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	environmentRepo := environment.NewRepo(c)

	return environmentRepo.ListEnvironments(ctx, in.ToParams())
}

type EnvironmentList struct {
	table *tui.Table[*client.Environment]
}

func NewEnvironmentList(ctx context.Context, input EnvironmentInput, selectEnvironment OnSelectFuncT[*client.Environment], opts ...tui.TableOption[*client.Environment]) *EnvironmentList {
	columns := []btable.Column{
		btable.NewFlexColumn("Name", "Name", 3).WithFiltered(true),
		btable.NewFlexColumn("Project", "Project", 3).WithFiltered(true),
		btable.NewFlexColumn("Protected", "Protected", 2).WithFiltered(true),
		btable.NewColumn("ID", "ID", 25).WithFiltered(true),
	}

	createRowFunc := func(env *client.Environment) btable.Row {
		return btable.NewRow(btable.RowData{
			"ID":          env.Id,
			"Name":        env.Name,
			"Project":     env.ProjectId,
			"Protected":   string(env.ProtectedStatus),
			"environment": env, // this will be hidden in the UI, but will be used to get the environment when selected
		})
	}

	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		e, ok := rows[0].Data["environment"].(*client.Environment)
		if !ok {
			return nil
		}

		return selectEnvironment(ctx, e)
	}

	t := tui.NewTable(
		columns,
		command.LoadCmd(ctx, LoadEnvironments, input),
		createRowFunc,
		onSelect,
		opts...,
	)

	return &EnvironmentList{
		table: t,
	}
}

func (pl *EnvironmentList) Init() tea.Cmd {
	return pl.table.Init()
}

func (pl *EnvironmentList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return pl.table.Update(msg)
}

func (pl *EnvironmentList) View() string {
	return pl.table.View()
}

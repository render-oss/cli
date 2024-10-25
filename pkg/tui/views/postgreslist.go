package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/postgres"
	postgrestui "github.com/renderinc/render-cli/pkg/postgres/tui"
	"github.com/renderinc/render-cli/pkg/project"
	"github.com/renderinc/render-cli/pkg/tui"
)

type PostgresList struct {
	table *tui.Table[*postgres.Model]
}

type OnSelectFuncT[T any] func(context.Context, T) tea.Cmd

func NewPostgresList(ctx context.Context, selectFunc OnSelectFuncT[*postgres.Model]) *PostgresList {
	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		p, ok := rows[0].Data["resource"].(*postgres.Model)
		if !ok {
			return nil
		}

		return selectFunc(ctx, p)
	}

	createRowFunc := func(p *postgres.Model) btable.Row {
		return postgrestui.Row(p)
	}

	t := tui.NewTable(
		postgrestui.Columns(),
		command.LoadCmd(ctx, listDatabases, PostgresInput{}),
		createRowFunc,
		onSelect,
	)

	return &PostgresList{
		table: t,
	}
}

type PostgresInput struct{}

func listDatabases(ctx context.Context, _ PostgresInput) ([]*postgres.Model, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)
	postgresRepo := postgres.NewRepo(c)

	postgresService := postgres.NewService(postgresRepo, environmentRepo, projectRepo)

	return postgresService.ListPostgres(ctx, &client.ListPostgresParams{})
}

func (pl *PostgresList) Init() tea.Cmd {
	return pl.table.Init()
}

func (pl *PostgresList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return pl.table.Update(msg)
}

func (pl *PostgresList) View() string {
	return pl.table.View()
}

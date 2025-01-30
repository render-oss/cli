package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/postgres"
	postgrestui "github.com/render-oss/cli/pkg/postgres/tui"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/tui"
)

type PostgresList struct {
	table *tui.Table[*postgres.Model]
}

type OnSelectFuncT[T any] func(context.Context, T) tea.Cmd

func NewPostgresList(ctx context.Context, selectFunc OnSelectFuncT[*postgres.Model], input PostgresInput, opts ...tui.TableOption[*postgres.Model]) *PostgresList {
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
		command.LoadCmd(ctx, listDatabases, input),
		createRowFunc,
		onSelect,
		opts...,
	)

	return &PostgresList{
		table: t,
	}
}

type PostgresInput struct {
	EnvironmentIDs []string
}

func listDatabases(ctx context.Context, input PostgresInput) ([]*postgres.Model, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)
	postgresRepo := postgres.NewRepo(c)

	postgresService := postgres.NewService(postgresRepo, environmentRepo, projectRepo)

	params := &client.ListPostgresParams{}
	if input.EnvironmentIDs != nil {
		params.EnvironmentId = &input.EnvironmentIDs
	}
	return postgresService.ListPostgres(ctx, params)
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

package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/keyvalue"
	keyvaluetui "github.com/render-oss/cli/pkg/keyvalue/tui"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/tui"
)

type KeyValueList struct {
	table *tui.Table[*keyvalue.Model]
}

func NewKeyValueList(ctx context.Context, selectFunc OnSelectFuncT[*keyvalue.Model], input KeyValueInput, opts ...tui.TableOption[*keyvalue.Model]) *KeyValueList {
	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		p, ok := rows[0].Data["resource"].(*keyvalue.Model)
		if !ok {
			return nil
		}

		return selectFunc(ctx, p)
	}

	createRowFunc := func(p *keyvalue.Model) btable.Row {
		return keyvaluetui.Row(p)
	}

	t := tui.NewTable(
		keyvaluetui.Columns(),
		command.LoadCmd(ctx, listKeyValues, input),
		createRowFunc,
		onSelect,
		opts...,
	)

	return &KeyValueList{
		table: t,
	}
}

type KeyValueInput struct {
	EnvironmentIDs []string
}

func listKeyValues(ctx context.Context, input KeyValueInput) ([]*keyvalue.Model, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)
	keyValueRepo := keyvalue.NewRepo(c)

	keyValueService := keyvalue.NewService(keyValueRepo, environmentRepo, projectRepo)

	params := &client.ListKeyValueParams{}
	if input.EnvironmentIDs != nil {
		params.EnvironmentId = &input.EnvironmentIDs
	}
	return keyValueService.ListKeyValue(ctx, params)
}

func (pl *KeyValueList) Init() tea.Cmd {
	return pl.table.Init()
}

func (pl *KeyValueList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return pl.table.Update(msg)
}

func (pl *KeyValueList) View() string {
	return pl.table.View()
}

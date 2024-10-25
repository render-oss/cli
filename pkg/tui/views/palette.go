package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/tui"
)

type PaletteCommand struct {
	Name        string
	Description string
	Action      func(ctx context.Context, args []string) tea.Cmd
}

type PaletteView struct {
	table *tui.Table[PaletteCommand]
}

const columnCommandKey = "Command"
const columnDescriptionKey = "Description"

func NewPaletteView(ctx context.Context, commands []PaletteCommand) *PaletteView {
	loadData := tui.TypedCmd[[]PaletteCommand](func() tea.Msg {
		return tui.LoadDataMsg[[]PaletteCommand]{Data: commands}
	})

	columns := []btable.Column{
		btable.NewColumn(columnCommandKey, "Command", 15).WithFiltered(true),
		btable.NewFlexColumn(columnDescriptionKey, "Description", 3),
	}

	createRowFunc := func(cmd PaletteCommand) btable.Row {
		return btable.NewRow(map[string]any{
			columnCommandKey:     cmd.Name,
			columnDescriptionKey: cmd.Description,
		})
	}

	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}
		selectedCommand, ok := rows[0].Data[columnCommandKey].(string)
		if !ok {
			return nil
		}

		for _, cmd := range commands {
			if cmd.Name == selectedCommand {
				return cmd.Action(ctx, nil)
			}
		}
		return nil
	}

	t := tui.NewTable(
		columns,
		loadData,
		createRowFunc,
		onSelect,
	)
	return &PaletteView{
		table: t,
	}
}

func (pv *PaletteView) Init() tea.Cmd {
	return pv.table.Init()
}

func (pv *PaletteView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return pv.table.Update(msg)
}

func (pv *PaletteView) View() string {
	return pv.table.View()
}

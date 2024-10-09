package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

type PaletteCommand struct {
	Name        string
	Description string
	Action      func(ctx context.Context, args []string) tea.Cmd
}

type PaletteCommandInput struct {
	Commands []PaletteCommand
}

func (p PaletteCommandInput) String() []string {
	var result []string
	for _, cmd := range p.Commands {
		result = append(result, cmd.Name)
	}
	return result
}

func loadCommandPalette(ctx context.Context, input PaletteCommandInput) ([]PaletteCommand, error) {
	return input.Commands, nil
}

var InteractiveCommandPalette = command.Wrap(
	paletteCmd,
	loadCommandPalette,
	renderPalette,
)

const columnCommandKey = "Command"
const columnDescriptionKey = "Description"

func renderPalette(
	ctx context.Context,
	loadData func(PaletteCommandInput) ([]PaletteCommand, error),
	in PaletteCommandInput,
) (tea.Model, error) {
	columns := []btable.Column{
		btable.NewColumn(columnCommandKey, "Command", 15).WithFiltered(true),
		btable.NewFlexColumn(columnDescriptionKey, "Description", 3),
	}

	commands, err := loadData(in)
	if err != nil {
		return nil, err
	}

	var rows []btable.Row
	for _, cmd := range commands {
		rows = append(rows, btable.NewRow(map[string]any{
			columnCommandKey:     cmd.Name,
			columnDescriptionKey: cmd.Description,
		}))
	}

	onSelect := func(data []btable.Row) tea.Cmd {
		if len(data) == 0 || len(data) > 1 {
			return nil
		}
		selectedCommand, ok := data[0].Data[columnCommandKey].(string)
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

	t := tui.NewNewTable(
		columns,
		rows,
		onSelect,
	)

	return t, nil
}

var paletteCmd = &cobra.Command{
	Use:    "palette",
	Short:  "Display a command palette",
	Hidden: true,
}

func init() {
	paletteCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input PaletteCommandInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}
		InteractiveCommandPalette(cmd.Context(), input)
		return nil
	}
	rootCmd.AddCommand(paletteCmd)
}

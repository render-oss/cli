package cmd

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
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

func renderPalette(
	ctx context.Context,
	loadData func(PaletteCommandInput) ([]PaletteCommand, error),
	in PaletteCommandInput,
) (tea.Model, error) {
	columns := []table.Column{
		{Title: "Command", Width: 20},
		{Title: "Description", Width: 50},
	}

	return tui.NewTableModel(
		"command palette",
		func() ([]PaletteCommand, error) { return loadData(in) },
		func(cmd PaletteCommand) table.Row { return []string{cmd.Name, cmd.Description} },
		func(cmd PaletteCommand) tea.Cmd { return cmd.Action(ctx, []string{}) },
		columns,
		func(cmd PaletteCommand, filter string) bool {
			return strings.Contains(strings.ToLower(cmd.Name), strings.ToLower(filter)) ||
				strings.Contains(strings.ToLower(cmd.Description), strings.ToLower(filter))
		},
		[]tui.CustomOption[PaletteCommand]{},
	), nil
}

var paletteCmd = &cobra.Command{
	Use:    "palette",
	Short:  "Display a command palette",
	Hidden: true,
}

func init() {
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
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

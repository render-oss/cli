package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/tui/views"
)

func InteractivePalette(ctx context.Context, commands []views.PaletteCommand, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, servicesCmd, breadcrumb, &views.PaletteCommand{},
		views.NewPaletteView(ctx, commands),
	)
}

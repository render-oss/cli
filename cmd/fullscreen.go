package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

// runFullScreenTUI is the launch point for full-screen Bubble Tea programs.
// Keeping launch here ensures telemetry records the attempt and every
// full-screen program uses the alternate screen and execution context.
func runFullScreenTUI(ctx context.Context, model tea.Model) (tea.Model, error) {
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx))

	if observation := executionObservationFromContext(ctx); observation != nil {
		observation.launchedFullScreenTUI = true
	}

	return program.Run()
}

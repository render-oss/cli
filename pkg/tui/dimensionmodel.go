package tui

import "github.com/charmbracelet/bubbletea"

// DimensionModel is an extension of tea.Model that implements a
// SetWidth and SetHeight method. This allows for models to handle their
// own sizing. For models that contain child models, their implementation
// to SetWidth and SetHeight should also call SetWidth and SetHeight on
// the child models and subtract out any padding or margins that the parent
// model may have.
//
// This allows for a more flexible and composable layout system, where each
// model is in charge of its own size and layout.
type DimensionModel interface {
	tea.Model
	SetWidth(int)
	SetHeight(int)
}

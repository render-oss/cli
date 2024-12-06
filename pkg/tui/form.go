package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

const (
	minHeight = 10
)

// Form is a wrapper around a huh form that implements the layout.DimensionModel interface
type Form struct {
	*huh.Form
}

func NewForm(huhForm *huh.Form) *Form {
	return &Form{Form: huhForm}
}

func (f *Form) Init() tea.Cmd {
	return f.Form.Init()
}

func (f *Form) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := f.Form.Update(msg)
	return f, cmd
}

func (f *Form) View() string {
	return f.Form.View()
}

func (f *Form) SetWidth(width int) {
	f.Form = f.Form.WithWidth(width)
}

func (f *Form) SetHeight(height int) {
	// Ensure the form is at least minHeight high
	// otherwise it may collapse some fields (like options)
	// and not expand even if the height eventually exceeds
	// minHeight
	f.Form = f.Form.WithHeight(max(height, minHeight))
}

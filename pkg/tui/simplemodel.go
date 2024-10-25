package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SimpleModel struct {
	loadFunc TypedCmd[string]
	message  string
	style    lipgloss.Style
}

func NewSimpleModel(loadFunc TypedCmd[string]) *SimpleModel {
	return &SimpleModel{
		loadFunc: loadFunc,
		style: lipgloss.NewStyle().
			Align(lipgloss.Center).
			Padding(1, 0, 1),
	}
}

func (m *SimpleModel) Init() tea.Cmd {
	return m.loadFunc.Unwrap()
}

func (m *SimpleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LoadDataMsg[string]:
		m.message = msg.Data
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *SimpleModel) View() string {
	if m.message == "" {
		return "Loading..."
	}
	return m.style.Render(m.message)
}

type SimpleLoadedMsg string

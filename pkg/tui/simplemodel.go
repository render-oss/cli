package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SimpleModel struct {
	loadFunc func() (string, error)
	message  string
	error    error
	style    lipgloss.Style
}

func NewSimpleModel(loadFunc func() (string, error)) *SimpleModel {
	return &SimpleModel{
		loadFunc: loadFunc,
		style: lipgloss.NewStyle().
			Align(lipgloss.Center).
			Padding(1, 0, 1),
	}
}

func (m *SimpleModel) Init() tea.Cmd {
	return func() tea.Msg {
		msg, err := m.loadFunc()
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return SimpleLoadedMsg(msg)
	}
}

func (m *SimpleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SimpleLoadedMsg:
		m.message = string(msg)
		return m, tea.Quit
	case ErrorMsg:
		m.error = msg.Err
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *SimpleModel) View() string {
	if m.error != nil {
		return NewErrorModel(m.error.Error()).View()
	}
	if m.message == "" {
		return "Loading..."
	}
	return m.style.Render(m.message)
}

type SimpleLoadedMsg string

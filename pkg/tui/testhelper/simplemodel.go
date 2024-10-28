package testhelper

import tea "github.com/charmbracelet/bubbletea"

type SimpleModel struct {
	Str string
}

func (m *SimpleModel) Init() tea.Cmd {
	return nil
}

func (m *SimpleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m *SimpleModel) View() string {
	return m.Str
}

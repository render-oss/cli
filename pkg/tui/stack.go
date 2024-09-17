package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type StackModel struct {
	stack []tea.Model
}

func NewStack() *StackModel {
	return &StackModel{}
}

func (m *StackModel) Push(model tea.Model) {
	m.stack = append(m.stack, model)
}

func (m *StackModel) Pop() {
	if len(m.stack) > 1 {
		m.stack = m.stack[:len(m.stack)-1]
	}
}

func (m *StackModel) Init() tea.Cmd {
	var cmd tea.Cmd
	for _, model := range m.stack {
		cmd = tea.Batch(cmd, model.Init())
	}
	return cmd
}

func (m *StackModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	if len(m.stack) > 0 {
		m.stack[len(m.stack)-1], cmd = m.stack[len(m.stack)-1].Update(msg)
	}

	return m, cmd
}

func (m *StackModel) View() string {
	if len(m.stack) == 0 {
		return ""
	}

	return m.stack[len(m.stack)-1].View()
}

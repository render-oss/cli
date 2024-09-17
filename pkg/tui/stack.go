package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type StackModel struct {
	stack []ModelWithCmd
}

type ModelWithCmd struct {
	Model tea.Model
	Cmd   string
}

func NewStack() *StackModel {
	return &StackModel{}
}

func (m *StackModel) Push(model ModelWithCmd) {
	m.stack = append(m.stack, model)
	model.Model.Init()
}

func (m *StackModel) Pop() {
	if len(m.stack) > 1 {
		m.stack = m.stack[:len(m.stack)-1]
	}
}

func (m *StackModel) Init() tea.Cmd {
	var cmd tea.Cmd
	for _, model := range m.stack {
		cmd = tea.Batch(cmd, model.Model.Init())
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
		m.stack[len(m.stack)-1].Model, cmd = m.stack[len(m.stack)-1].Model.Update(msg)
	}

	return m, cmd
}

func (m *StackModel) View() string {
	if len(m.stack) == 0 {
		return ""
	}

	return lipgloss.JoinVertical(lipgloss.Left, m.stack[len(m.stack)-1].Cmd, m.stack[len(m.stack)-1].Model.View())
}

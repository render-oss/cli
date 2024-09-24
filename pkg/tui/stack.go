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

// ErrorMsg quits the program after displaying an error message
type ErrorMsg struct {
	Err error
}

// QuitMsg quits the program after displaying a message
type QuitMsg struct {
	Message string
}

// ClearScreenMsg is a message that clears the screen before rendering the next message
type ClearScreenMsg struct {
	NextMsg tea.Msg
}

func NewStack() *StackModel {
	return &StackModel{}
}

func (m *StackModel) Push(model ModelWithCmd) {
	m.stack = append(m.stack, model)
	model.Model.Init()
}

func (m *StackModel) Pop() tea.Cmd {
	if len(m.stack) > 1 {
		m.stack = m.stack[:len(m.stack)-1]
		return nil
	}
	return tea.Quit
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
		case "esc":
			return m, m.Pop()
		}
	case ClearScreenMsg:
		m.stack = m.stack[:0]
		return m, func() tea.Msg {
			return msg.NextMsg
		}
	case ErrorMsg:
		return m, tea.Sequence(
			tea.Println(msg.Err.Error(), tea.Quit()),
		)
	case QuitMsg:
		if msg.Message != "" {
			return m, tea.Sequence(
				tea.Println(msg.Message),
				tea.Quit,
			)
		}
		return m, tea.Quit
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

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var stackHeaderStyle = lipgloss.NewStyle().BorderStyle(lipgloss.ThickBorder()).BorderBottom(true).BorderTop(true)

type StackModel struct {
	stack []ModelWithCmd

	width  int
	height int
}

type ModelWithCmd struct {
	Model tea.Model
	Cmd   string
}

// ErrorMsg quits the program after displaying an error message
type ErrorMsg struct {
	Err error
}

// DoneMsg quits the program after displaying a message
type DoneMsg struct {
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
	if len(m.stack) == 0 {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
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
			tea.Println(msg.Err.Error()),
			tea.Quit,
		)
	case DoneMsg:
		cmd := m.Pop()
		if cmd != nil {
			return m, cmd
		}

		return m, m.Init()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update the message for subcomponents to exclude the header
		msg.Height -= lipgloss.Height(m.header())
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

	return lipgloss.JoinVertical(lipgloss.Left, m.header(), m.stack[len(m.stack)-1].Model.View())
}

func (m *StackModel) header() string {
	emptyStyle := lipgloss.NewStyle()

	escText := "Esc: Quit"
	if len(m.stack) > 1 {
		escText = "Esc: Previous command"
	}
	escVal := emptyStyle.Render(escText)

	globalCmds := "Ctrl+C: Quit"
	globalCmdsVal := emptyStyle.Render(globalCmds)

	cmdStyle := lipgloss.NewStyle()
	cmdText := fmt.Sprintf("Current Command: %s", m.stack[len(m.stack)-1].Cmd)

	cmdWidth := lipgloss.Width(cmdText)
	paddingLeft := (m.width-cmdWidth)/2 - lipgloss.Width(escVal)
	cmdStyle = cmdStyle.PaddingLeft(paddingLeft)

	paddingRight := (m.width-cmdWidth)/2 - lipgloss.Width(globalCmdsVal)
	cmdStyle = cmdStyle.PaddingRight(paddingRight)

	cmdVal := cmdStyle.Render(cmdText)

	bar := lipgloss.JoinHorizontal(lipgloss.Top,
		escVal,
		cmdVal,
		globalCmdsVal,
	)

	return stackHeaderStyle.Render(bar)
}

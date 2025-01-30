package tui

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/render-oss/cli/pkg/config"
	renderstyle "github.com/render-oss/cli/pkg/style"
)

type UserFacingError struct {
	Title   string
	Message string
	Err     error
}

func (u UserFacingError) Error() string {
	return u.Err.Error()
}

var stackHeaderStyle = lipgloss.NewStyle().MarginTop(1).MarginBottom(1)
var stackInfoStyle = lipgloss.NewStyle().Foreground(renderstyle.ColorBreadcrumb).Bold(true)

type StackModel struct {
	loadingSpinner *spinner.Model
	stack          []ModelWithCmd

	width      int
	height     int
	done       func(msg tea.Msg) (tea.Model, tea.Cmd)
	loadingMsg string
}

type ModelWithCmd struct {
	Model      tea.Model
	Cmd        string
	Breadcrumb string
}

type StackSizeMsg struct {
	Width  int
	Height int
	Top    int
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

type BackMsg struct {
	Handled bool
}

func NewStack() *StackModel {
	return &StackModel{}
}

func (m *StackModel) WithDone(f func(tea.Msg) (tea.Model, tea.Cmd)) {
	m.done = f
}

func newSpinner() *spinner.Model {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &spin
}

func (m *StackModel) Push(model ModelWithCmd) tea.Cmd {
	m.stack = append(m.stack, model)
	return tea.Sequence(model.Model.Init(), tea.WindowSize())
}

func (m *StackModel) PushError(err error) tea.Cmd {
	return m.Push(ModelWithCmd{Model: NewErrorModel(err)})
}

func (m *StackModel) Pop() *ModelWithCmd {
	if len(m.stack) > 0 {
		popped := m.stack[len(m.stack)-1]
		m.stack = m.stack[:len(m.stack)-1]
		return &popped
	}
	return nil
}

func (m *StackModel) Init() tea.Cmd {
	if len(m.stack) == 0 {
		return tea.Quit
	}

	var cmd tea.Cmd
	for _, model := range m.stack {
		cmd = tea.Batch(cmd, model.Model.Init())
	}
	return tea.Batch(cmd)
}

func (m *StackModel) back() tea.Cmd {
	m.Pop()
	if len(m.stack) == 0 {
		return tea.Quit
	}
	return m.Init()
}

func (m *StackModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(m.stack) == 0 {
		return m, tea.Quit
	}

	subMsg := msg

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyCtrlD:
			return m, m.back()
		case tea.KeyEsc:
			subMsg = &BackMsg{}
		case tea.KeyCtrlS:
			// copy command to clipboard
			if len(m.stack) > 0 {
				err := clipboard.WriteAll(m.stack[len(m.stack)-1].Cmd)
				if err != nil {
					m.PushError(fmt.Errorf("Failed to copy command to clipboard"))
				}
			}
		}
	case ClearScreenMsg:
		m.stack = m.stack[:0]
		return m, func() tea.Msg {
			return msg.NextMsg
		}
	case LoadingDataMsg:
		m.loadingSpinner = newSpinner()
		if msg.LoadingMsgTmpl != "" {
			m.loadingMsg = msg.LoadingMsgTmpl
		}
		return m, tea.Batch(m.loadingSpinner.Tick, msg.Cmd)
	case DoneLoadingDataMsg:
		m.loadingSpinner = nil
		m.loadingMsg = ""
		return m, nil
	case ErrorMsg:
		// We want to keep the breadcrumb but pop the model that caused the error
		// this way ctrl+D doesn't go back to the command that caused the error
		previous := m.Pop()

		var breadcrumb string
		if previous != nil {
			breadcrumb = previous.Breadcrumb
		}

		return m, m.Push(ModelWithCmd{
			Model:      NewErrorModel(msg.Err),
			Breadcrumb: breadcrumb,
		})
	case spinner.TickMsg:
		if m.loadingSpinner != nil {
			spin, cmd := m.loadingSpinner.Update(msg)
			m.loadingSpinner = &spin
			return m, cmd
		}
	case DoneMsg:
		m.Pop()
		if len(m.stack) == 0 {
			if m.done != nil {
				return m.done(msg)
			}

			return m, tea.Sequence(
				tea.Println(msg.Message),
				tea.Quit,
			)
		}

		return m, m.Init()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update the message for subcomponents to exclude the header
		subMsg = m.StackSizeMsg()
	}

	var cmd tea.Cmd
	if len(m.stack) > 0 {
		m.stack[len(m.stack)-1].Model, cmd = m.stack[len(m.stack)-1].Model.Update(subMsg)
	}

	if backMsg, ok := subMsg.(*BackMsg); ok {
		if !backMsg.Handled {
			return m, m.back()
		}
	}

	return m, cmd
}

func (m *StackModel) StackSizeMsg() StackSizeMsg {
	return StackSizeMsg{
		Width:  m.width,
		Height: m.height - lipgloss.Height(m.header()) - lipgloss.Height(m.footer()),
		Top:    lipgloss.Height(m.header()),
	}
}

func (m *StackModel) View() string {
	if m.loadingSpinner != nil {
		loadingTmpl := "%s Loading..."
		if m.loadingMsg != "" {
			loadingTmpl = m.loadingMsg
		}
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, fmt.Sprintf(loadingTmpl, m.loadingSpinner.View()))
	}

	if len(m.stack) == 0 {
		return ""
	}

	return lipgloss.JoinVertical(lipgloss.Left, m.header(), m.stack[len(m.stack)-1].Model.View(), m.footer())
}

func (m *StackModel) header() string {
	var breadCrumbs []string

	workspace, _ := config.WorkspaceName()
	if workspace != "" {
		breadCrumbs = append(breadCrumbs, stackInfoStyle.Render(workspace))
	}

	for _, model := range m.stack {
		if model.Breadcrumb != "" {
			breadCrumbs = append(breadCrumbs, stackInfoStyle.Render(model.Breadcrumb))
		}
	}

	return stackHeaderStyle.Render(strings.Join(breadCrumbs, " > "))
}

func (m *StackModel) footer() string {
	quitCommand := fmt.Sprintf("%s: Quit", renderstyle.CommandKey.Render("[Ctrl+C]"))
	prevCommand := fmt.Sprintf("%s: Back", renderstyle.CommandKey.Render("[Esc]"))
	saveToClipboard := fmt.Sprintf("%s: Copy command to clipboard", renderstyle.CommandKey.Render("[Ctrl+S]"))

	var commands []string
	commands = append(commands, quitCommand)

	if len(m.stack) > 1 {
		commands = append(commands, prevCommand)
	}

	if m.stack[len(m.stack)-1].Cmd != "" {
		commands = append(commands, saveToClipboard)
	}

	return strings.Join(commands, "  ")
}

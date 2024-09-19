package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()
)

func NewLogModel(loadFunc func() ([]string, error)) *LogModel {
	return &LogModel{
		loadFunc: loadFunc,
	}
}

type LogModel struct {
	loadFunc func() ([]string, error)
	content  []string
	ready    bool
	viewport viewport.Model
}

func (m *LogModel) loadData() tea.Msg {
	data, err := m.loadFunc()
	if err != nil {
		return loadLogsErrMsg(err)
	}
	return loadLogsMsg(data)
}

type loadLogsMsg []string
type loadLogsErrMsg error

func (m *LogModel) Init() tea.Cmd {
	return m.loadData
}

func (m *LogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case loadLogsErrMsg:
		return m, tea.Quit
	case loadLogsMsg:
		m.content = msg
		m.viewport.SetContent(strings.Join(m.content, "\n"))
	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			// Since this program is using the full size of the viewport we
			// need to wait until we've received the window dimensions before
			// we can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we wait for them
			// here.
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.SetContent(strings.Join(m.content, "\n"))
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *LogModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m *LogModel) headerView() string {
	title := titleStyle.Render("Logs For")
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m *LogModel) footerView() string {
	return strings.Repeat("─", max(0, m.viewport.Width))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

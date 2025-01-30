package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/render-oss/cli/pkg/client"
	lclient "github.com/render-oss/cli/pkg/client/logs"
)

type LogResult struct {
	Logs       *client.Logs200Response
	LogChannel <-chan *lclient.Log
}

type LoadFunc func() (*client.Logs200Response, <-chan *lclient.Log, error)

func NewLogModel(loadFunc TypedCmd[*LogResult]) *LogModel {
	return &LogModel{
		help:      help.New(),
		loadFunc:  loadFunc,
		scrollBar: NewScrollBarModel(1, 0),
		viewport:  viewport.New(0, 0),
		state:     logStateLoading,
	}
}

type logState string

const (
	logStateLoading logState = "loading"
	logStateLoaded  logState = "loaded"
)

type LogModel struct {
	loadFunc  TypedCmd[*LogResult]
	content   []string
	state     logState
	viewport  viewport.Model
	scrollBar *ScrollBarModel
	help      help.Model

	windowWidth  int
	windowHeight int
	top          int

	logChan <-chan *lclient.Log
}

type appendLogsMsg struct {
	log *lclient.Log
}

type logChanClose struct{}

var timeStyle = lipgloss.NewStyle().PaddingRight(2)

func formatLogs(logs []lclient.Log) []string {
	var formattedLogs []string
	for _, log := range logs {
		formattedLogs = append(formattedLogs, lipgloss.JoinHorizontal(
			lipgloss.Top,
			timeStyle.Render(log.Timestamp.Format(time.DateTime)),
			log.Message,
		))
	}

	return formattedLogs
}

func (m *LogModel) readFromChannel(ch <-chan *lclient.Log) tea.Cmd {
	return func() tea.Msg {
		select {
		case log, ok := <-ch:
			if !ok {
				m.logChan = nil
				return logChanClose{}
			}
			return appendLogsMsg{log: log}
		}
	}
}

func (m *LogModel) Init() tea.Cmd {
	return tea.Batch(m.loadFunc.Unwrap(), m.scrollBar.Init(), tea.WindowSize())
}

func (m *LogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case LoadDataMsg[*LogResult]:
		if msg.Data.Logs != nil {
			m.content = formatLogs(msg.Data.Logs.Logs)
		} else {
			m.content = []string{}
		}

		m.logChan = msg.Data.LogChannel
		if m.logChan != nil {
			cmds = append(cmds, m.readFromChannel(m.logChan))
		}
		m.viewport.SetContent(strings.Join(m.content, "\n"))
		m.state = logStateLoaded
	case logChanClose:
		m.content = append(m.content, "Websocket connection closed, no more logs will be displayed. Press 'r' to reload.")
		m.viewport.SetContent(strings.Join(m.content, "\n"))
		m.viewport.GotoBottom()
	case appendLogsMsg:
		m.content = append(m.content, formatLogs([]lclient.Log{*msg.log})...)
		isAtBottom := m.viewport.AtBottom()
		m.viewport.SetContent(strings.Join(m.content, "\n"))
		if isAtBottom {
			m.viewport.GotoBottom()
		}
		if m.logChan != nil {
			cmds = append(cmds, m.readFromChannel(m.logChan))
		}
	case tea.KeyMsg:
		switch msg.Type {
		default:
			if k := msg.String(); k == "r" && m.logChan == nil {
				cmds = append(cmds, tea.Batch(m.loadFunc.Unwrap()))
			}
		}
	}

	m.scrollBar.ScrollPercent(m.viewport.ScrollPercent())

	m.scrollBar, cmd = m.scrollBar.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *LogModel) SetWidth(width int) {
	m.windowWidth = width
	m.setViewPortSize()
}

func (m *LogModel) SetHeight(height int) {
	m.windowHeight = height
	m.setViewPortSize()
}

func (m *LogModel) setViewPortSize() {
	scrollBarWidth := 1

	m.viewport.Height = m.windowHeight
	m.viewport.YPosition = 0
	m.viewport.Width = m.windowWidth - scrollBarWidth

	m.scrollBar.SetHeight(m.viewport.Height - 1)
}

func (m *LogModel) KeyBinds() []key.Binding {
	return (&keyMapWrapper{m.viewport.KeyMap}).ShortHelp()
}

func (m *LogModel) View() string {
	if m.state != logStateLoaded {
		return "\n  Loading Logs..."
	}
	logContent := m.viewport.View()

	if m.content == nil || len(m.content) == 0 {
		emptyStateMessage := "No logs to show."
		if m.logChan != nil {
			emptyStateMessage = "No logs to show. New log entries that match your search parameters will appear here."
		}
		logContent = lipgloss.Place(m.viewport.Width, m.viewport.Height, lipgloss.Center, lipgloss.Center, emptyStateMessage)
	}

	logView := lipgloss.JoinHorizontal(
		lipgloss.Top,
		logContent,
		m.scrollBar.View(),
	)

	return logView
}

type keyMapWrapper struct {
	keyMap viewport.KeyMap
}

func (k *keyMapWrapper) ShortHelp() []key.Binding {
	return []key.Binding{
		k.keyMap.Down,
		k.keyMap.Up,
		k.keyMap.PageDown,
		k.keyMap.PageUp,
		k.keyMap.HalfPageDown,
		k.keyMap.HalfPageUp,
	}
}

func (k *keyMapWrapper) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			k.keyMap.Down,
			k.keyMap.Up,
			k.keyMap.PageDown,
			k.keyMap.PageUp,
			k.keyMap.HalfPageDown,
			k.keyMap.HalfPageUp,
		},
	}
}

package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/renderinc/render-cli/pkg/client"
	lclient "github.com/renderinc/render-cli/pkg/client/logs"
)

const (
	searchWidth = 60
)

var viewportSylte = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, true, false)
var logStyle = lipgloss.NewStyle().Padding(1, 2, 2, 2)
var filterStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, true, false, false)

type LoadFunc func() (*client.Logs200Response, <-chan *lclient.Log, error)

func NewLogModel(filter *FilterModel, loadFunc LoadFunc) *LogModel {
	return &LogModel{
		help:        help.New(),
		loadFunc:    loadFunc,
		searching:   false,
		filterModel: filter,
		errorModel:  NewErrorModel(""),
		scrollBar:   NewScrollBarModel(1, 0),
		viewport:    viewport.New(0, 0),
	}
}

type logState string

const (
	logStateLoading logState = "loading"
	logStateLoaded  logState = "loaded"
	logStateError   logState = "error"
)

type LogModel struct {
	loadFunc    LoadFunc
	content     []string
	state       logState
	viewport    viewport.Model
	scrollBar   *ScrollBarModel
	filterModel *FilterModel
	errorModel  *ErrorModel
	help        help.Model

	windowWidth  int
	windowHeight int
	top          int
	searching    bool

	logChan <-chan *lclient.Log
}

func (m *LogModel) loadData() tea.Msg {
	logs, logChan, err := m.loadFunc()
	if err != nil {
		return loadLogsErrMsg(err)
	}
	m.logChan = logChan
	return loadLogsMsg{data: logs}
}

type loadLogsMsg struct {
	data *client.Logs200Response
}

type appendLogsMsg struct {
	log *lclient.Log
}

type logChanClose struct{}
type loadLogsErrMsg error

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
	return tea.Batch(m.loadData, m.filterModel.Init(), m.errorModel.Init(), m.scrollBar.Init(), tea.WindowSize())
}

func (m *LogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// Handle keyboard and mouse events in the viewport
	if m.searching {
		m.filterModel, cmd = m.filterModel.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case loadLogsErrMsg:
		m.errorModel.DisplayError = msg.Error()
		m.state = logStateError
	case loadLogsMsg:
		if msg.data != nil {
			m.content = formatLogs(msg.data.Logs)
		} else {
			m.content = []string{}
		}
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
			if k := msg.String(); k == "/" {
				m.searching = !m.searching
				m.setViewPortSize()
			} else if k == "r" && m.logChan == nil {
				cmds = append(cmds, tea.Batch(m.loadData))
			}
		}
	case StackSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		m.top = msg.Top
		m.setViewPortSize()
	}

	m.scrollBar.ScrollPercent(m.viewport.ScrollPercent())

	m.scrollBar, cmd = m.scrollBar.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *LogModel) setViewPortSize() {
	stylingHeight := logStyle.GetPaddingTop() + logStyle.GetPaddingBottom() + logStyle.GetBorderTopSize() + logStyle.GetBorderBottomSize()
	stylingWidth := logStyle.GetPaddingRight() + logStyle.GetPaddingLeft() + logStyle.GetBorderLeftSize() + logStyle.GetBorderRightSize()
	searchWindowWidth := min(searchWidth, m.windowWidth)
	scrollBarWidth := 1

	m.viewport.Height = m.windowHeight - stylingHeight - m.top
	m.viewport.YPosition = stylingHeight + m.top
	if m.searching {
		m.viewport.Width = m.windowWidth - searchWindowWidth - stylingWidth - scrollBarWidth
		m.filterModel.SetWidth(searchWindowWidth)
		m.filterModel.SetHeight(m.top, m.viewport.Height)
	} else {
		m.viewport.Width = m.windowWidth - stylingWidth - scrollBarWidth
	}

	m.scrollBar.SetHeight(m.viewport.Height - 1)
}

func (m *LogModel) View() string {
	if m.state == logStateError {
		return m.errorModel.View()
	}

	if m.state == logStateLoading {
		return "\n  Loading Logs..."
	}
	logContent := viewportSylte.Render(m.viewport.View())

	if m.content == nil || len(m.content) == 0 {
		emptyStateMessage := "No logs to show."
		if m.logChan != nil {
			emptyStateMessage = "No logs to show. New log entries that match your search parameters will appear here."
		}
		logContent = lipgloss.Place(m.viewport.Width, m.viewport.Height, lipgloss.Center, lipgloss.Center, emptyStateMessage)
	}

	logView := logStyle.Render(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.JoinVertical(lipgloss.Left, logContent, m.help.View(&keyMapWrapper{m.viewport.KeyMap})),
			m.scrollBar.View(),
		),
	)

	if m.searching {
		return lipgloss.JoinHorizontal(lipgloss.Center, filterStyle.Render(m.filterModel.View()), logView)
	}

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

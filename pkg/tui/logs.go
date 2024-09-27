package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	searchWidth              = 60
	commandDescriptionHeight = 1
)

var viewportStyle = lipgloss.NewStyle().Padding(0, 2).Border(lipgloss.NormalBorder())

func NewLogModel(filter *FilterModel, loadFunc func() ([]string, error)) *LogModel {
	return &LogModel{
		loadFunc:    loadFunc,
		searching:   false,
		filterModel: filter,
		errorModel:  NewErrorModel(""),
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
	loadFunc    func() ([]string, error)
	content     []string
	state       logState
	viewport    viewport.Model
	filterModel *FilterModel
	errorModel  *ErrorModel

	windowWidth  int
	windowHeight int
	searching    bool
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
	return tea.Batch(m.loadData, m.filterModel.Init(), m.errorModel.Init(), tea.WindowSize())
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
		m.content = msg
		m.viewport.SetContent(strings.Join(m.content, "\n"))
		m.state = logStateLoaded
	case tea.KeyMsg:
		switch msg.Type {
		default:
			if k := msg.String(); k == "/" {
				m.searching = !m.searching
				m.setViewPortSize()
			}
		}
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		m.setViewPortSize()
	}

	return m, tea.Batch(cmds...)
}

func (m *LogModel) setViewPortSize() {
	stylingHeight := viewportStyle.GetPaddingTop() + viewportStyle.GetPaddingBottom() + viewportStyle.GetBorderTopSize() + viewportStyle.GetBorderBottomSize()
	stylingWidth := viewportStyle.GetPaddingRight() + viewportStyle.GetPaddingLeft() + viewportStyle.GetBorderLeftSize() + viewportStyle.GetBorderRightSize()
	searchWindowWidth := min(searchWidth, m.windowWidth)

	m.viewport.Height = m.windowHeight - stylingHeight - commandDescriptionHeight
	m.viewport.YPosition = stylingHeight + commandDescriptionHeight
	if m.searching {
		m.viewport.Width = m.windowWidth - searchWindowWidth - stylingWidth
		m.filterModel.SetWidth(searchWindowWidth)
		m.filterModel.SetHeight(m.viewport.Height)
	} else {
		m.viewport.Width = m.windowWidth - stylingWidth
	}
}

func (m *LogModel) View() string {
	if m.state == logStateError {
		return m.errorModel.View()
	}

	if m.state == logStateLoading {
		return "\n  Loading Logs..."
	}
	logView := viewportStyle.Render(m.viewport.View())

	if m.searching {
		return lipgloss.JoinHorizontal(lipgloss.Center, m.filterModel.View(), logView)
	}

	return logView
}

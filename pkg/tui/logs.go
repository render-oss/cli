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
		direction: lclient.Backward, // default direction
	}
}

// Sets the function to call when more logs need to be loaded
func (m *LogModel) SetLoadMoreFunc(f func(startTime, endTime *time.Time) tea.Cmd) {
	m.loadMoreFunc = f
}

// Sets the log query direction for pagination logic
func (m *LogModel) SetDirection(direction lclient.LogDirection) {
	m.direction = direction
}

type logState string

const (
	logStateLoading logState = "loading"
	logStateLoaded  logState = "loaded"
)

type LogModel struct {
	loadFunc     TypedCmd[*LogResult]
	loadMoreFunc func(startTime, endTime *time.Time) tea.Cmd
	content      []string
	state        logState
	viewport     viewport.Model
	scrollBar    *ScrollBarModel
	help         help.Model

	windowWidth  int
	windowHeight int
	top          int

	logChan <-chan *lclient.Log

	// Pagination state
	hasMore         bool
	nextStartTime   *time.Time
	nextEndTime     *time.Time
	direction       lclient.LogDirection
	isLoadingMore   bool
	initialLoadDone bool // Track if initial load is complete to prevent immediate auto-fetch
	lastYOffset     int  // Track last Y offset to detect actual scroll changes
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

	// Check if user is trying to scroll up while at top before the viewport has
	// updated; this is always the case when scrolling up after the initial load
	wasAtTop := m.viewport.AtTop()
	userTriedToScrollUp := false

	// Detect key presses that would scroll up
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(keyMsg, m.viewport.KeyMap.Up) ||
			key.Matches(keyMsg, m.viewport.KeyMap.HalfPageUp) ||
			key.Matches(keyMsg, m.viewport.KeyMap.PageUp) {
			if wasAtTop && m.state == logStateLoaded {
				userTriedToScrollUp = true
			}
		}
	}

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	// Check if we should load more logs based on scroll position
	// - when scrolling up, load older logs (prepend above)
	// - when scrolling down, load newer logs (append below)
	if m.state == logStateLoaded && m.hasMore && !m.isLoadingMore && m.loadMoreFunc != nil && m.logChan == nil {
		// Mark initial load as done on first update after content is loaded
		if !m.initialLoadDone && len(m.content) > 0 {
			m.initialLoadDone = true
			m.lastYOffset = m.viewport.YOffset
		}

		if m.initialLoadDone {
			shouldLoadMore := false

			if m.direction == lclient.Backward {
				// Backward direction queries most recent logs first
				shouldLoadMore = (m.viewport.AtTop() && m.viewport.YOffset != m.lastYOffset) || userTriedToScrollUp
			} else {
				// Forward direction queries oldest logs first
				shouldLoadMore = m.viewport.AtBottom() && m.viewport.YOffset != m.lastYOffset
			}

			if shouldLoadMore {
				m.isLoadingMore = true
				cmds = append(cmds, m.loadMoreFunc(m.nextStartTime, m.nextEndTime))
			}

			m.lastYOffset = m.viewport.YOffset
		}
	}

	switch msg := msg.(type) {
	case LoadDataMsg[*LogResult]:
		if msg.Data.Logs != nil {
			// If loading more logs, append or prepend based on direction
			if m.isLoadingMore {
				newContent := formatLogs(msg.Data.Logs.Logs)

				if m.direction == lclient.Backward {
					// Backward direction: paginated scroll prepends older logs
					m.content = append(newContent, m.content...)
					// Set content first so the viewport knows the new valid
					// range, then adjust the viewport to maintain the current
					// scroll position. It's fine for SetContent to be called
					// again outside of this block.
					m.viewport.SetContent(strings.Join(m.content, "\n"))
					m.viewport.SetYOffset(m.viewport.YOffset + len(newContent))
				} else {
					// Forward direction: paginated scroll appends newer logs
					m.content = append(m.content, newContent...)
				}
				m.isLoadingMore = false
			} else {
				// Initial load
				m.content = formatLogs(msg.Data.Logs.Logs)
			}

			// Update pagination state
			m.hasMore = msg.Data.Logs.HasMore
			m.nextStartTime = &msg.Data.Logs.NextStartTime
			m.nextEndTime = &msg.Data.Logs.NextEndTime
		} else {
			m.content = []string{}
			m.hasMore = false
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

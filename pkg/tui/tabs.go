package tui

import (
	"strings"

	renderstyle "github.com/renderinc/cli/pkg/style"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	horizontalLine    = "─"
	verticalLine      = "│"
	topRightCorner    = "┐"
	bottomLeftCorner  = "└"
	bottomRightCorner = "┘"
	rightT            = "├"
	bottomT           = "┴"
)

var (
	nextTab        = key.NewBinding(key.WithKeys("shift+right"), key.WithHelp("shift+right", "next tab"))
	previousTab    = key.NewBinding(key.WithKeys("shift+left"), key.WithHelp("shift+left", "previous tab"))
	tabKeyBindings = []key.Binding{nextTab, previousTab}
)

type Tab struct {
	Name    string
	Content DimensionModel
}

// Adapted from the tabs bubbletea example: https://github.com/charmbracelet/bubbletea/tree/main/examples/tabs
type TabModel struct {
	Tabs      []*Tab
	activeTab int
	width     int
}

func NewTabModel(tabs []*Tab) *TabModel {
	return &TabModel{
		Tabs: tabs,
	}
}

func (m *TabModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, tab := range m.Tabs {
		cmds = append(cmds, tab.Content.Init())
	}
	return tea.Batch(cmds...)
}

func (m *TabModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "shift+right":
			m.activeTab = min(m.activeTab+1, len(m.Tabs)-1)
			return m, nil
		case "shift+left":
			m.activeTab = max(m.activeTab-1, 0)
			return m, nil
		}
	}

	var cmd tea.Cmd
	_, cmd = m.Tabs[m.activeTab].Content.Update(msg)

	return m, cmd
}

func (m *TabModel) SetWidth(width int) {
	m.width = width

	for _, tab := range m.Tabs {
		tab.Content.SetWidth(width - windowStyle.GetHorizontalFrameSize())
	}
}

func (m *TabModel) SetHeight(height int) {
	innerHeight := height - (lipgloss.Height(m.Header()) + windowStyle.GetVerticalFrameSize())
	for _, tab := range m.Tabs {
		tab.Content.SetHeight(innerHeight)
	}
}

func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft = left
	border.Bottom = middle
	border.BottomRight = right
	return border
}

var (
	inactiveTabBorder = tabBorderWithBottom(bottomT, horizontalLine, bottomT)
	activeTabBorder   = tabBorderWithBottom(bottomRightCorner, " ", bottomLeftCorner)
	inactiveTabStyle  = lipgloss.NewStyle().Border(inactiveTabBorder, true).BorderForeground(renderstyle.ColorBorder).Padding(0, 1)
	activeTabStyle    = inactiveTabStyle.Border(activeTabBorder, true)
	windowStyle       = lipgloss.NewStyle().BorderForeground(renderstyle.ColorBorder).Padding(2, 0).Border(lipgloss.NormalBorder()).UnsetBorderTop()
)

func (m *TabModel) Header() string {
	var renderedTabs []string

	for i, t := range m.Tabs {
		var tabStyle lipgloss.Style
		isFirst := i == 0
		isLast := i == len(m.Tabs)-1
		isActive := i == m.activeTab

		if isActive {
			tabStyle = activeTabStyle
		} else {
			tabStyle = inactiveTabStyle
		}
		border, _, _, _, _ := tabStyle.GetBorder()
		if isFirst && isActive {
			border.BottomLeft = verticalLine
		} else if isFirst && !isActive {
			border.BottomLeft = rightT
		} else if isLast && isActive {
			border.BottomRight = bottomLeftCorner
		} else if isLast && !isActive {
			border.BottomRight = bottomT
		}
		tabStyle = tabStyle.BorderForeground(renderstyle.ColorBorder).Border(border)
		renderedTabs = append(renderedTabs, tabStyle.Render(t.Name))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	gap := lipgloss.NewStyle().BorderForeground(renderstyle.ColorBorder).
		Foreground(renderstyle.ColorBorder).Render(strings.Repeat("─", max(0, m.width-lipgloss.Width(row)-1)) + topRightCorner)
	row = row + gap + "\n"

	return row
}

func (m *TabModel) View() string {
	doc := strings.Builder{}

	row := m.Header()
	doc.WriteString(row)
	doc.WriteString(windowStyle.Render(m.Tabs[m.activeTab].Content.View()))
	return doc.String()
}

func (m *TabModel) KeyBinds() []key.Binding {
	return tabKeyBindings
}

func (m *TabModel) CurrentTab() *Tab {
	return m.Tabs[m.activeTab]
}

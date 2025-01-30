package layouts

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/render-oss/cli/pkg/tui"
)

// SidebarLayout is a layout that renders a sidebar, content, and footer.
// The sidebar can be toggled on and off, and the width of the sidebar and
// height of the footer can be adjusted.
//
// All the child models are expected to implement the DimensionModel interface,
// and should therefore handle their own sizing.
type SidebarLayout struct {
	sidebar tui.DimensionModel
	content tui.DimensionModel
	footer  tui.DimensionModel

	sidebarVisible bool
	sidebarWidth   int
	footerHeight   int

	width  int
	height int
}

func NewSidebarLayout(sidebar, content, footer tui.DimensionModel) *SidebarLayout {
	return &SidebarLayout{
		sidebar: sidebar,
		content: content,
		footer:  footer,
	}
}

func (l *SidebarLayout) SetSidebarWidth(width int) {
	l.sidebarWidth = width

	l.updateSizes()
}

func (l *SidebarLayout) SetFooterHeight(height int) {
	l.footerHeight = height

	l.updateSizes()
}

func (l *SidebarLayout) SetSidebarVisible(visible bool) {
	l.sidebarVisible = visible

	l.updateSizes()
}

func (l *SidebarLayout) calculatedSidebarWidth() int {
	if l.sidebarVisible {
		return l.sidebarWidth
	}

	return 0
}

func (l *SidebarLayout) updateSizes() {
	l.sidebar.SetWidth(l.calculatedSidebarWidth())
	l.content.SetWidth(l.width - l.calculatedSidebarWidth())
	l.footer.SetWidth(l.width)

	l.sidebar.SetHeight(l.height - l.footerHeight)
	l.content.SetHeight(l.height - l.footerHeight)
	l.footer.SetHeight(l.footerHeight)
}

func (l *SidebarLayout) Init() tea.Cmd {
	return tea.Batch(l.sidebar.Init(), l.content.Init(), l.footer.Init())
}

func (l *SidebarLayout) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tui.StackSizeMsg:
		l.width = msg.Width
		l.height = msg.Height

		l.updateSizes()
	}

	if l.sidebarVisible {
		_, cmd := l.sidebar.Update(msg)
		return l, cmd
	}

	_, cmd := l.content.Update(msg)
	return l, cmd
}

func (l *SidebarLayout) View() string {
	if l.sidebarVisible {
		return lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Top, l.sidebar.View(), l.content.View()),
			l.footer.View(),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		l.content.View(),
		l.footer.View(),
	)
}

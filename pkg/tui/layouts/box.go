package layouts

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/renderinc/cli/pkg/tui"
)

// BoxLayout is a simple layout that renders a single child model with a
// specified style. It implements the DimensionModel interface so it can
// be a convenient way to wrap a model with a style.
type BoxLayout struct {
	style   lipgloss.Style
	content tui.DimensionModel
}

func NewBoxLayout(style lipgloss.Style, content tui.DimensionModel) *BoxLayout {
	return &BoxLayout{style: style, content: content}
}

func (l *BoxLayout) Init() tea.Cmd {
	return l.content.Init()
}

func (l *BoxLayout) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := l.content.Update(msg)
	return l, cmd
}

func (l *BoxLayout) View() string {
	return l.style.Render(l.content.View())
}

func (l *BoxLayout) SetWidth(width int) {
	l.content.SetWidth(width - l.style.GetHorizontalFrameSize())
}

func (l *BoxLayout) SetHeight(height int) {
	l.content.SetHeight(height - l.style.GetVerticalFrameSize())
}

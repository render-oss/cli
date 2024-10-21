package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var emptyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
var fullStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))

type ScrollBarModel struct {
	height  int
	percent float64
}

func NewScrollBarModel(
	height int,
	percent float64,
) *ScrollBarModel {
	m := &ScrollBarModel{
		height:  height,
		percent: percent,
	}

	return m
}

func (m *ScrollBarModel) Init() tea.Cmd {
	return nil
}

func (m *ScrollBarModel) Update(_ tea.Msg) (*ScrollBarModel, tea.Cmd) {
	return m, nil
}

func (m *ScrollBarModel) View() string {
	var strs []string
	highlightedIndex := int(m.percent * float64(m.height))
	for i := 0; i <= m.height; i++ {
		if i == highlightedIndex {
			strs = append(strs, fullStyle.Render("█"))
		} else {
			strs = append(strs, emptyStyle.Render("█"))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, strs...)
}

func (m *ScrollBarModel) SetHeight(height int) {
	m.height = height
}

func (m *ScrollBarModel) ScrollPercent(percent float64) {
	m.percent = percent
}

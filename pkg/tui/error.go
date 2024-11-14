package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var style = lipgloss.NewStyle().
	Bold(true).
	Align(lipgloss.Center).
	Foreground(lipgloss.Color("#BA0D35")).
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("#BA0D35")).
	PaddingTop(2).
	PaddingBottom(2).
	PaddingRight(1).
	PaddingLeft(1).
	MarginBottom(1)

type ErrorModel struct {
	DisplayError string
	width        int
	height       int
}

func NewErrorModel(
	displayError string,
) *ErrorModel {
	m := &ErrorModel{
		DisplayError: displayError,
	}

	return m
}

func (m *ErrorModel) Init() tea.Cmd {
	return nil
}

func (m *ErrorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case StackSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m *ErrorModel) View() string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, style.Render(m.DisplayError))
}

package tui

import (
	"errors"

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
	Err    error
	width  int
	height int
}

func NewErrorModel(
	err error,
) *ErrorModel {
	m := &ErrorModel{
		Err: err,
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
	var title string
	var message string

	userFacingError := &UserFacingError{}
	if errors.As(m.Err, userFacingError) {
		title = userFacingError.Title
		message = userFacingError.Message
	} else {
		message = m.Err.Error()
	}

	var interior string

	if title == "" {
		interior = lipgloss.JoinVertical(lipgloss.Center, message)
	} else {
		interior = lipgloss.JoinVertical(lipgloss.Center, title, message)
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, style.Render(interior))
}

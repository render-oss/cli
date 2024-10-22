package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var dialogBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#874BFD")).
	Padding(1, 0).
	BorderTop(true).
	BorderLeft(true).
	BorderRight(true).
	BorderBottom(true)

var buttonStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFF7DB")).
	Background(lipgloss.Color("#888B7E")).
	Padding(0, 3).
	MarginTop(1)

var activeButtonStyle = buttonStyle.
	Foreground(lipgloss.Color("#FFF7DB")).
	Background(lipgloss.Color("#F25D94")).
	MarginRight(2).
	Underline(true)

type ConfirmModel struct {
	onConfirm func() tea.Cmd
	onCancel  func() tea.Cmd

	message string

	width  int
	height int
}

func NewConfirmModel(
	message string,
	onConfirm func() tea.Cmd,
	onCancel func() tea.Cmd,
) *ConfirmModel {
	m := &ConfirmModel{
		message:   message,
		onConfirm: onConfirm,
		onCancel:  onCancel,
	}

	return m
}

func (m *ConfirmModel) Init() tea.Cmd {
	return nil
}

func (m *ConfirmModel) Update(msg tea.Msg) (*ConfirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case StackSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m *ConfirmModel) View() string {
	okButton := activeButtonStyle.Render("Yes")
	cancelButton := buttonStyle.Render("Maybe")

	question := lipgloss.NewStyle().Width(50).Align(lipgloss.Center).Render("Are you sure you want to eat marmalade?")
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, okButton, cancelButton)
	ui := lipgloss.JoinVertical(lipgloss.Center, question, buttons)

	dialog := lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialogBoxStyle.Render(ui),
	)

	return dialog
}

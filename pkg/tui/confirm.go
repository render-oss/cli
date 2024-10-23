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
	MarginTop(1).
	MarginRight(2).
	MarginLeft(2)

var activeButtonStyle = buttonStyle.
	Foreground(lipgloss.Color("#FFF7DB")).
	Background(lipgloss.Color("#F25D94")).
	MarginRight(2).
	MarginLeft(2).
	Underline(true)

type ConfirmModel struct {
	onConfirm func() tea.Cmd
	onCancel  func() tea.Cmd

	selected bool

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
		selected:  false,
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
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyRight:
			m.selected = false
		case tea.KeyLeft:
			m.selected = true
		case tea.KeyEnter:
			if m.selected {
				return m, m.onConfirm()
			} else {
				return m, m.onCancel()
			}
		default:
			switch msg.String() {
			case "y":
				return m, m.onConfirm()
			case "n":
				return m, m.onCancel()
			}
		}
	}
	return m, nil
}

func (m *ConfirmModel) View() string {
	var okButton, cancelButton string
	if m.selected {
		okButton = activeButtonStyle.Render("Yes (y)")
		cancelButton = buttonStyle.Render("No (n)")
	} else {
		okButton = buttonStyle.Render("Yes (y)")
		cancelButton = activeButtonStyle.Render("No (n)")
	}

	question := lipgloss.NewStyle().Width(50).Align(lipgloss.Center).Render(m.message)
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, okButton, cancelButton)
	ui := lipgloss.JoinVertical(lipgloss.Center, question, buttons)

	dialog := lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialogBoxStyle.Render(ui),
	)

	return dialog
}

type ModelWithConfirm struct {
	confirming bool
	confirm    *ConfirmModel
	model      tea.Model
}

func NewModelWithConfirm(model tea.Model, message string) *ModelWithConfirm {
	mc := &ModelWithConfirm{
		confirming: true,
		model:      model,
	}
	confirm := &ConfirmModel{
		message: message,
		onConfirm: func() tea.Cmd {
			mc.confirming = false
			return model.Init()
		},
		onCancel: func() tea.Cmd {
			return func() tea.Msg { return DoneMsg{} }
		},
	}

	mc.confirm = confirm

	return mc
}

func (m *ModelWithConfirm) Init() tea.Cmd {
	return m.confirm.Init()
}

func (m *ModelWithConfirm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.confirming {
		m.confirm, cmd = m.confirm.Update(msg)
		return m, cmd
	}

	m.model, cmd = m.model.Update(msg)
	return m, cmd
}

func (m *ModelWithConfirm) View() string {
	if m.confirming {
		return m.confirm.View()
	}

	return m.model.View()
}

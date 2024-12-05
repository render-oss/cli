package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var headerStyle = lipgloss.NewStyle().Margin(1).Border(lipgloss.NormalBorder(), false, false, true, false)
var header = headerStyle.Render("Update filters")

type FilterModel struct {
	form   *huh.Form
	search func(*huh.Form) tea.Cmd
}

func NewFilterModel(form *huh.Form, search func(*huh.Form) tea.Cmd) *FilterModel {
	return &FilterModel{
		form:   form,
		search: search,
	}
}
func (m *FilterModel) SetWidth(width int) {
	m.form = m.form.WithWidth(width)
}

func (m *FilterModel) SetHeight(top int, height int) {
	m.form = m.form.WithHeight(height - lipgloss.Height(header) - top)
}

func (m *FilterModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *FilterModel) Update(msg tea.Msg) (*FilterModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			return m, m.search(m.form)
		default:
			// Don't allow the user to type a "/" in the filter form, this is used to close the filter
			if k := msg.String(); k == "/" {
				return m, nil
			}
		}
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	return m, cmd
}

func (m *FilterModel) View() string {
	return lipgloss.JoinVertical(lipgloss.Left, header, m.form.View())
}

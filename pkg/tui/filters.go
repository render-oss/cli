package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

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

func (m *FilterModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *FilterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			return m, m.search(m.form)
		}
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	return m, cmd
}

func (m *FilterModel) View() string {
	return m.form.View()
}

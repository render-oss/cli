package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
)

var styleSubtle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))

const defaultMinHeight = 25
const defaultMaxWidth = 100

type NewTable struct {
	Model    table.Model
	onSelect func(data []table.Row) tea.Cmd

	tableWidth int
}

func NewNewTable(
	columns []table.Column,
	rows []table.Row,
	onSelect func(data []table.Row) tea.Cmd,
) *NewTable {
	t := &NewTable{
		Model: table.New(columns).
			Filtered(true).
			Focused(true).
			WithPageSize(25).
			WithBaseStyle(lipgloss.NewStyle().Align(lipgloss.Left)).
			WithTargetWidth(defaultMaxWidth).
			WithRows(rows),
		onSelect: onSelect,
	}

	return t
}

func (t *NewTable) Init() tea.Cmd {
	return t.Model.Init()
}

func (t *NewTable) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return t, t.onSelect([]table.Row{t.Model.HighlightedRow()})
		}
	}

	var cmd tea.Cmd
	t.Model, cmd = t.Model.Update(msg)
	return t, cmd
}

func (t *NewTable) View() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		t.Model.View(),
		// todo: add footer with available actions
	)
}

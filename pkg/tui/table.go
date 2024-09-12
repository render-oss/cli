package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type TableModel[T any] struct {
	loading bool

	columns    []table.Column
	formatFunc func(T) table.Row
	loadFunc   func() ([]T, error)
	selectFunc func(T) tea.Cmd

	data       []T
	name       string
	table      table.Model
	spinner    spinner.Model
	SelectedID string
}

func NewTableModel[T any](name string, loadFunc func() ([]T, error), formatFunc func(T) table.Row, selectFunc func(T) tea.Cmd, columns []table.Column) *TableModel[T] {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return &TableModel[T]{
		name:       name,
		formatFunc: formatFunc,
		loadFunc:   loadFunc,
		selectFunc: selectFunc,
		columns:    columns,
		spinner:    spin,
		loading:    true,
	}
}

func (m *TableModel[T]) loadDeploys() tea.Msg {
	data, err := m.loadFunc()
	if err != nil {
		return loadedErrMsg(err)
	}

	m.data = data
	return loadDataMsg[T](data)
}

type loadDataMsg[T any] []T
type loadedErrMsg error

func (m *TableModel[T]) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadDeploys)
}

type Column struct {
	Title string
	Width int
}

func (m *TableModel[T]) setTableData(msg loadDataMsg[T]) *TableModel[T] {
	var rows []table.Row
	for _, d := range msg {
		rows = append(rows, m.formatFunc(d))
	}

	tuiColumns := make([]table.Column, len(m.columns))
	for i, c := range m.columns {
		tuiColumns[i] = table.Column{Title: c.Title, Width: c.Width}
	}

	t := table.New(
		table.WithColumns(tuiColumns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)
	m.table = t
	m.loading = false
	return m
}

func (m *TableModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case loadDataMsg[T]:
		return m.setTableData(msg), nil
	case loadedErrMsg:
		m.loading = false
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "enter":
			for _, datum := range m.data {
				if m.formatFunc(datum)[0] == m.table.SelectedRow()[0] {
					return m, m.selectFunc(datum)
				}
			}
		}
	}
	if m.loading {
		m.spinner, cmd = m.spinner.Update(msg)
	} else {
		m.table, cmd = m.table.Update(msg)
	}
	return m, cmd
}

func (m *TableModel[T]) View() string {
	if m.loading {
		return fmt.Sprintf("\n\n   %s Loading %s...\n\n", m.spinner.View(), m.name)
	}
	return baseStyle.Render(m.table.View()) + "\n"
}

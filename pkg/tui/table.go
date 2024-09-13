package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type TableModel[T any] struct {
	loading bool

	columns    []table.Column
	formatFunc func(T) table.Row
	loadFunc   func() ([]T, error)
	selectFunc func(T) tea.Cmd

	filterFunc    func(T, string) bool
	currentFilter string

	data         []T
	filteredData []T
	name         string
	table        table.Model
	spinner      spinner.Model
	SelectedID   string
	searchInput  textinput.Model
	searching    bool
}

func NewTableModel[T any](
	name string,
	loadFunc func() ([]T, error),
	formatFunc func(T) table.Row,
	selectFunc func(T) tea.Cmd,
	columns []table.Column,
	filterFunc func(T, string) bool,
) *TableModel[T] {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 156
	ti.Width = 20

	t := table.New(
		table.WithColumns(columns),
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

	return &TableModel[T]{
		name:        name,
		formatFunc:  formatFunc,
		loadFunc:    loadFunc,
		selectFunc:  selectFunc,
		filterFunc:  filterFunc,
		columns:     columns,
		spinner:     spin,
		searchInput: ti,
		table:       t,
		loading:     true,
	}
}

func (m *TableModel[T]) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadData)
}

func (m *TableModel[T]) loadData() tea.Msg {
	data, err := m.loadFunc()
	if err != nil {
		return loadedErrMsg(err)
	}
	return loadDataMsg[T](data)
}

type loadDataMsg[T any] []T
type loadedErrMsg error

func (m *TableModel[T]) setTableData(msg loadDataMsg[T]) {
	m.data = msg
	m.filteredData = msg
	m.updateTableRows()
	m.loading = false
}

func (m *TableModel[T]) updateTableRows() {
	var rows []table.Row
	for _, d := range m.filteredData {
		rows = append(rows, m.formatFunc(d))
	}
	m.table.SetRows(rows)
}

func (m *TableModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case loadDataMsg[T]:
		m.setTableData(msg)
		return m, nil
	case loadedErrMsg:
		m.loading = false
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.searching {
				m.searching = false
				m.searchInput.Blur()
				m.currentFilter = ""
				m.filteredData = m.data
				m.updateTableRows()
			} else if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "/":
			if !m.searching {
				m.searching = true
				m.searchInput.Focus()
				return m, textinput.Blink
			}
		case "enter":
			if m.searching {
				m.searching = false
				m.searchInput.Blur()
			}
			return m, m.selectCurrentRow()
		case "up", "down":
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		}
	}

	if m.loading {
		m.spinner, cmd = m.spinner.Update(msg)
	} else if m.searching {
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.currentFilter = m.searchInput.Value()
		m.filteredData = m.filterData(m.currentFilter)
		m.updateTableRows()
		// Ensure table keeps focus for navigation
		m.table.Focus()
	} else {
		m.table, cmd = m.table.Update(msg)
	}
	return m, cmd
}

func (m *TableModel[T]) selectCurrentRow() tea.Cmd {
	if len(m.table.SelectedRow()) > 0 {
		for _, datum := range m.filteredData {
			if m.formatFunc(datum)[0] == m.table.SelectedRow()[0] {
				return m.selectFunc(datum)
			}
		}
	}
	return nil
}

func (m *TableModel[T]) filterData(query string) []T {
	if query == "" {
		return m.data
	}
	var filtered []T
	for _, item := range m.data {
		if m.filterFunc(item, strings.ToLower(query)) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (m *TableModel[T]) View() string {
	if m.loading {
		return fmt.Sprintf("\n\n   %s Loading %s...\n\n", m.spinner.View(), m.name)
	}

	var view strings.Builder

	// Render the table
	view.WriteString(baseStyle.Render(m.table.View()))
	view.WriteString("\n\n")

	// Render the search input and current filter at the bottom
	if m.searching {
		view.WriteString(fmt.Sprintf("Search: %s\n", m.searchInput.View()))
	}

	return view.String()
}

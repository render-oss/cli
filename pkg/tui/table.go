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

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type CustomOption[T any] struct {
	Key      string
	Title    string
	Function func(T) tea.Cmd
}

type tableState string

const (
	tableStateLoading tableState = "loading"
	tableStateLoaded  tableState = "loaded"
	tableStateError   tableState = "error"
)

type TableModel[T any] struct {
	name          string
	tableState    tableState
	data          []T
	filteredData  []T
	table         table.Model
	spinner       spinner.Model
	errorModel    *ErrorModel
	searchInput   textinput.Model
	searching     bool
	currentFilter string

	columns    []table.Column
	formatFunc func(T) table.Row
	loadFunc   func() ([]T, error)
	selectFunc func(T) tea.Cmd
	filterFunc func(T, string) bool

	customOptions []CustomOption[T]

	actionStyle lipgloss.Style
}

func NewTableModel[T any](
	name string,
	loadFunc func() ([]T, error),
	formatFunc func(T) table.Row,
	selectFunc func(T) tea.Cmd,
	columns []table.Column,
	filterFunc func(T, string) bool,
	customOptions []CustomOption[T],
) *TableModel[T] {
	m := &TableModel[T]{
		name:          name,
		formatFunc:    formatFunc,
		loadFunc:      loadFunc,
		selectFunc:    selectFunc,
		filterFunc:    filterFunc,
		columns:       columns,
		tableState:    tableStateLoading,
		actionStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		customOptions: customOptions,
		errorModel:    NewErrorModel(""),
	}

	return m
}

func (m *TableModel[T]) initSpinner() {
	m.spinner = spinner.New()
	m.spinner.Spinner = spinner.Dot
	m.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
}

func (m *TableModel[T]) initSearchInput() {
	m.searchInput = textinput.New()
	m.searchInput.Placeholder = "Search..."
	m.searchInput.CharLimit = 156
	m.searchInput.Width = 20
}

func (m *TableModel[T]) initTable() {
	m.table = table.New(
		table.WithColumns(m.columns),
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
	m.table.SetStyles(s)
}

func (m *TableModel[T]) Init() tea.Cmd {
	m.initSpinner()
	m.initSearchInput()
	m.initTable()

	return tea.Batch(m.spinner.Tick, m.loadData, m.errorModel.Init())
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
	m.tableState = tableStateLoaded
}

func (m *TableModel[T]) updateTableRows() {
	rows := make([]table.Row, len(m.filteredData))
	for i, d := range m.filteredData {
		rows[i] = m.formatFunc(d)
	}
	m.table.SetRows(rows)
}

func (m *TableModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadDataMsg[T]:
		m.setTableData(msg)
		return m, nil
	case loadedErrMsg:
		m.tableState = tableStateError
		m.errorModel.DisplayError = msg.Error()
		return m, nil
	case tea.KeyMsg:
		if m.searching {
			return m.updateSearching(msg)
		}
		return m.handleKeyMsg(msg)
	}

	return m.updateComponents(msg)
}

func (m *TableModel[T]) executeCustomOption(option CustomOption[T]) (tea.Model, tea.Cmd) {
	selectedRow := m.table.SelectedRow()
	if len(selectedRow) > 0 {
		for _, datum := range m.filteredData {
			if m.formatFunc(datum)[0] == selectedRow[0] {
				return m, option.Function(datum)
			}
		}
	}
	return m, nil
}

func (m *TableModel[T]) updateSearching(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)

	switch msg.String() {
	case "enter":
		return m.handleEnter()
	case "esc":
		return m.handleEsc()
	case "up", "down":
		m.table, cmd = m.table.Update(msg)
	default:
		m.currentFilter = m.searchInput.Value()
		m.filteredData = m.filterData(m.currentFilter)
		m.updateTableRows()
	}

	return m, cmd
}

func (m *TableModel[T]) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.handleEsc()
	case "/":
		return m.handleSlash()
	case "enter":
		return m.handleEnter()
	case "up", "down":
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	default:
		for _, option := range m.customOptions {
			if msg.String() == option.Key {
				return m.executeCustomOption(option)
			}
		}
	}
	return m, nil
}

func (m *TableModel[T]) handleEsc() (tea.Model, tea.Cmd) {
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
	return m, nil
}

func (m *TableModel[T]) handleSlash() (tea.Model, tea.Cmd) {
	if !m.searching {
		m.searching = true
		m.searchInput.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

func (m *TableModel[T]) handleEnter() (tea.Model, tea.Cmd) {
	if m.searching {
		m.searching = false
		m.searchInput.Blur()
	}
	return m, m.selectCurrentRow()
}

func (m *TableModel[T]) updateComponents(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.tableState {
	case tableStateLoading:
		m.spinner, cmd = m.spinner.Update(msg)
	case tableStateLoaded:
		m.table, cmd = m.table.Update(msg)
	case tableStateError:
		m.errorModel, cmd = m.errorModel.Update(msg)
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
	if m.tableState == tableStateError {
		return m.errorModel.View()
	}

	if m.tableState == tableStateLoading {
		return fmt.Sprintf("\n\n   %s Loading %s...\n\n", m.spinner.View(), m.name)
	}

	var view strings.Builder
	view.WriteString(baseStyle.Render(m.table.View()))
	view.WriteString("\n\n")

	if m.searching {
		view.WriteString(fmt.Sprintf("Search: %s\n", m.searchInput.View()))
	} else {
		view.WriteString(m.renderActions())
	}

	return view.String()
}

func (m *TableModel[T]) renderActions() string {
	actions := []string{
		m.actionStyle.Render("/ Search"),
	}
	for _, option := range m.customOptions {
		actions = append(actions, m.actionStyle.Render(fmt.Sprintf("%s %s", option.Key, option.Title)))
	}
	return strings.Join(actions, "  ")
}

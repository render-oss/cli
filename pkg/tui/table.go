package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
)

var styleSubtle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))

const defaultMaxWidth = 100

var defaultFilterCustomOption = CustomOption{
	Key:   "/",
	Title: "Filter",
}

type CustomOption struct {
	Key      string
	Title    string
	Function func(row table.Row) tea.Cmd
}

func (o CustomOption) String() string {
	return fmt.Sprintf("[%s] %s", o.Key, o.Title)
}

type Table[T any] struct {
	Model         table.Model
	onSelect      func(rows []table.Row) tea.Cmd
	customOptions []CustomOption

	loadData    func() ([]T, error)
	createRow   func(T) table.Row
	data        []T
	columns     []table.Column

	tableWidth int

	loading bool
	spinner spinner.Model
}

type TableOption[T any] func(*Table[T])

func WithCustomOptions[T any](options []CustomOption) TableOption[T] {
	return func(t *Table[T]) {
		t.customOptions = options
	}
}

func NewTable[T any](
	columns []table.Column,
	loadData func() ([]T, error),
	createRow func(T) table.Row,
	onSelect func(rows []table.Row) tea.Cmd,
	tableOptions ...TableOption[T],
) *Table[T] {
	t := &Table[T]{
		Model: table.New(columns).
			Filtered(true).
			Focused(true).
			WithPageSize(25).
			WithBaseStyle(lipgloss.NewStyle().Align(lipgloss.Left)).
			WithTargetWidth(defaultMaxWidth),
		onSelect:    onSelect,
		loadData:    loadData,
		createRow:   createRow,
		columns:     columns,
	}

	for _, option := range tableOptions {
		option(t)
	}

	return t
}

func (t *Table[T]) Init() tea.Cmd {
	t.loading = true
	t.initSpinner()


	return tea.Batch(t.spinner.Tick, t.loadDataCmd(), t.Model.Init())
}

func (t *Table[T]) loadDataCmd() tea.Cmd {
	return func() tea.Msg {
		data, err := t.loadData()
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return loadDataMsg[T]{data: data}
	}
}

type loadDataMsg[T any] struct {
	data []T
}

func (t *Table[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case loadDataMsg[T]:
		t.data = msg.data
		rows := make([]table.Row, len(t.data))
		for i, item := range t.data {
			rows[i] = t.createRow(item)
		}
		t.Model = t.Model.WithRows(rows)
		t.loading = false
		return t, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return t, t.onSelect([]table.Row{t.Model.HighlightedRow()})
		default:
			if !t.Model.GetIsFilterInputFocused() {
				for _, option := range t.customOptions {
					if msg.String() == option.Key {
						t.loading = true
						return t, option.Function(t.Model.HighlightedRow())
					}
				}
			}
		}
	case spinner.TickMsg:
		if t.loading {
			t.spinner, cmd = t.spinner.Update(msg)
			return t, cmd
		}
	}

	if t.loading {
		return t, t.spinner.Tick
	}

	t.Model, cmd = t.Model.Update(msg)
	return t, cmd
}

func (t *Table[T]) View() string {
	if t.loading {
		return fmt.Sprintf("\n\n   %s Loading...\n\n", t.spinner.View())
	}

	var footer string

	var options []string
	if len(t.customOptions) > 0 {
		for _, option := range t.customOptions {
			options = append(options, styleSubtle.Render(option.String()))
		}
	}

	options = append(options, styleSubtle.Render(defaultFilterCustomOption.String()))

	footer = lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(options, " "))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		t.Model.View(),
		footer,
	)
}

func (t *Table[T]) initSpinner() {
	t.spinner = spinner.New()
	t.spinner.Spinner = spinner.Dot
	t.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
}
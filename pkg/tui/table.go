package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
)

var styleSubtle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))

const defaultMaxWidth = 100

type CustomOption struct {
	Key      string
	Title    string
	Function func(row table.Row) tea.Cmd
}

func (o CustomOption) String() string {
	return fmt.Sprintf("[%s] %s", o.Key, o.Title)
}

type OnInitFunc func(tableModel *Table) tea.Cmd

type Table struct {
	Model         table.Model
	onSelect      func(data []table.Row) tea.Cmd
	customOptions []CustomOption

	onReInit     OnInitFunc
	shouldReInit bool

	tableWidth int

	loading bool
	spinner spinner.Model
}

type TableOption func(*Table)

func WithCustomOptions(options []CustomOption) func(*Table) {
	return func(t *Table) {
		t.customOptions = options
	}
}

func WithOnReInit(onInit OnInitFunc) func(*Table) {
	return func(t *Table) {
		t.onReInit = onInit
	}
}

func NewTable(
	columns []table.Column,
	rows []table.Row,
	onSelect func(data []table.Row) tea.Cmd,
	tableOptions ...TableOption,
) *Table {
	t := &Table{
		Model: table.New(columns).
			Filtered(true).
			Focused(true).
			WithPageSize(25).
			WithBaseStyle(lipgloss.NewStyle().Align(lipgloss.Left)).
			WithTargetWidth(defaultMaxWidth).
			WithRows(rows),
		onSelect: onSelect,
	}

	for _, option := range tableOptions {
		option(t)
	}

	return t
}

func (t *Table) Init() tea.Cmd {
	t.initSpinner()

	if t.shouldReInit && t.onReInit != nil {
		t.shouldReInit = false
		return tea.Batch(t.spinner.Tick, tea.Sequence(t.onReInit(t), t.Model.Init()))
	}

	return tea.Batch(t.spinner.Tick, t.Model.Init())
}

func (t *Table) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return t, t.onSelect([]table.Row{t.Model.HighlightedRow()})
		default:
			if !t.Model.GetIsFilterInputFocused() {
				for _, option := range t.customOptions {
					if msg.String() == option.Key {
						t.shouldReInit = true
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

func (t *Table) View() string {
	if t.loading {
		return fmt.Sprintf("\n\n   %s Loading...\n\n", t.spinner.View())
	}

	var footer string
	if len(t.customOptions) > 0 {
		var options []string
		for _, option := range t.customOptions {
			options = append(options, styleSubtle.Render(option.String()))
		}
		footer = lipgloss.JoinHorizontal(lipgloss.Left, options...)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		t.Model.View(),
		footer,
	)
}

func (t *Table) UpdateRows(rows []table.Row) {
	t.Model = t.Model.WithRows(rows)
	t.loading = false
}

func (t *Table) initSpinner() {
	t.spinner = spinner.New()
	t.spinner.Spinner = spinner.Dot
	t.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
}

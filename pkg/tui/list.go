package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ListItem interface {
	Title() string
	Description() string
	FilterValue() string
	Height() int
}

type List[T any] struct {
	list           list.Model
	items          []T
	selected       *T
	title          string
	loadData       func() ([]T, error)
	makeListItem   func(T) ListItem
	loading        bool
	spinner        spinner.Model
	err            error
	windowHeight   int
	windowWidth    int
	maxWidth       int
}

func NewList[T any](title string, loadData func() ([]T, error), makeListItem func(T) ListItem) *List[T] {
	delegate := list.NewDefaultDelegate()

	l := list.New([]list.Item{}, delegate, 0, 0) // Size is updated in Init

	l.Title = title
	l.SetShowStatusBar(false)

	// Filtering isn't well-supported for lists with color styling or for multi-value FilterValues. We will likely
	// need to avoid using filtering or find a way to swap out the filtering implementation.
	l.SetFilteringEnabled(false)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &List[T]{
		list:         l,
		title:        title,
		loadData:     loadData,
		makeListItem: makeListItem,
		loading:      true,
		spinner:      s,
		maxWidth:     300,
	}
}

func (m *List[T]) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.load,
		tea.WindowSize(),
	)
}

func (m *List[T]) load() tea.Msg {
	items, err := m.loadData()
	if err != nil {
		return errMsg{err}
	}
	return loadedMsg{items}
}

type loadedMsg struct {
	items interface{}
}

type errMsg struct {
	err error
}

func (m *List[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case StackSizeMsg:
		m.windowHeight = msg.Height
		m.windowWidth = msg.Width
		m.updateListSize()
	case loadedMsg:
		m.loading = false
		m.items = msg.items.([]T)
		listItems := make([]list.Item, len(m.items))
		for i, item := range m.items {
			listItems[i] = m.makeListItem(item)
		}
		m.list.SetItems(listItems)
		m.updateListSize()
		return m, nil
	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *List[T]) updateListSize() {
	availableHeight := m.windowHeight


	listWidth := min(m.windowWidth, m.maxWidth)
	m.list.SetSize(listWidth, availableHeight)

	delegate := list.NewDefaultDelegate()

	maxItemHeight := 0
	for _, item := range m.items {
		listItem := m.makeListItem(item)
		if listItem.Height() > maxItemHeight {
			maxItemHeight = listItem.Height()
		}
	}
	delegate.SetHeight(maxItemHeight)

	delegate.SetSpacing(1)
	m.list.SetDelegate(delegate)
}

func (m *List[T]) View() string {
	if m.loading {
		return m.spinner.View() + " Loading..."
	}
	if m.err != nil {
		return "Error: " + m.err.Error()
	}
	return m.list.View()
}

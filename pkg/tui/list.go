package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	renderstyle "github.com/render-oss/cli/pkg/style"
)

type ListItem interface {
	Title() string
	Description() string
	FilterValue() string
	Height() int
}

type List[T any] struct {
	list         list.Model
	items        []T
	selected     *T
	title        string
	loadData     TypedCmd[[]T]
	makeListItem func(T) ListItem
	windowHeight int
	windowWidth  int
	maxWidth     int
	onSelect     func(ListItem) tea.Cmd

	hasMoreData bool
}

type ListOption[T any] func(*List[T])

func WithOnSelect[T any](onSelect func(ListItem) tea.Cmd) ListOption[T] {
	return func(l *List[T]) {
		l.onSelect = onSelect
	}
}

func NewList[T any](
	title string,
	loadData TypedCmd[[]T],
	makeListItem func(T) ListItem,
	opts ...ListOption[T],
) *List[T] {
	delegate := list.NewDefaultDelegate()

	l := list.New([]list.Item{}, delegate, 0, 0) // Size is updated in Init

	if title != "" {
		l.Styles.Title = renderstyle.TitleBlock
		l.Title = title
	} else {
		l.SetShowTitle(false)
	}
	l.SetShowStatusBar(false)

	// Filtering isn't well-supported for lists with color styling or for multi-value FilterValues. We will likely
	// need to avoid using filtering or find a way to swap out the filtering implementation.
	l.SetFilteringEnabled(false)

	result := &List[T]{
		list:         l,
		title:        title,
		loadData:     loadData,
		makeListItem: makeListItem,
		maxWidth:     300,
	}

	for _, opt := range opts {
		opt(result)
	}

	return result
}

func (m *List[T]) Init() tea.Cmd {
	return tea.Batch(
		m.loadData.Unwrap(),
		tea.WindowSize(),
	)
}

func (m *List[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case StackSizeMsg:
		m.windowHeight = msg.Height
		m.windowWidth = msg.Width
		m.updateListSize()
	case LoadDataMsg[[]T]:
		m.items = msg.Data
		listItems := make([]list.Item, len(m.items))
		for i, item := range m.items {
			listItems[i] = m.makeListItem(item)
		}
		m.list.SetItems(listItems)
		m.updateListSize()
		m.hasMoreData = msg.HasMore
		return m, nil
	case tea.KeyMsg:
		if isKeyDown(msg, m.list) && m.hasMoreData && m.list.Index() == len(m.items)-1 {
			// set hasMoreData to false so we don't load multiple times
			// if the down button is continuously pressed
			m.hasMoreData = false
			return m, m.loadData.Unwrap()
		}

		if msg.String() == "enter" && m.onSelect != nil {
			selectedItem := m.list.SelectedItem()
			if selectedItem != nil {
				return m, m.onSelect(selectedItem.(ListItem))
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func isKeyDown(msg tea.KeyMsg, list list.Model) bool {
	return key.Matches(msg.Type, list.KeyMap.CursorDown) ||
		key.Matches(msg.Type, list.KeyMap.NextPage) ||
		key.Matches(msg.Type, list.KeyMap.GoToEnd)
}

func (m *List[T]) updateListSize() {
	availableHeight := m.windowHeight

	listWidth := min(m.windowWidth, m.maxWidth)
	m.list.SetSize(listWidth, availableHeight)

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.BorderForeground(renderstyle.ColorInfo)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.BorderForeground(renderstyle.ColorInfo)

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
	return m.list.View()
}

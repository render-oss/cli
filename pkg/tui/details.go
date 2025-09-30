package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	renderstyle "github.com/render-oss/cli/pkg/style"
)

type DetailsModel[T any] struct {
	loadData     TypedCmd[T]
	makeDetails  func(T) []KeyValue
	windowHeight int
	windowWidth  int

	data T
}

type KeyValue struct {
	Key   string
	Value string
}

func NewDetailsModel[T any](
	title string,
	loadData TypedCmd[T],
	makeDetails func(T) []KeyValue,
) *DetailsModel[T] {
	return &DetailsModel[T]{
		loadData:    loadData,
		makeDetails: makeDetails,
	}
}

func (m *DetailsModel[T]) Init() tea.Cmd {
	return tea.Batch(
		m.loadData.Unwrap(),
		tea.WindowSize(),
	)
}

func (m *DetailsModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LoadDataMsg[T]:
		m.data = msg.Data
		return m, nil
	case StackSizeMsg:
		m.windowHeight = msg.Height
		m.windowWidth = msg.Width
	}
	return m, nil
}

func (m *DetailsModel[T]) View() string {
	keyValues := m.makeDetails(m.data)
	message := ""
	for _, keyValue := range keyValues {
		message += renderstyle.FormatKeyValue(keyValue.Key, keyValue.Value) + "\n"
	}

	return lipgloss.Place(m.windowWidth, m.windowHeight, lipgloss.Left, lipgloss.Top, message)
}

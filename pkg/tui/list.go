package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type listModel struct {
	list     list.Model
	choice   string
	quitting bool
}

func NewList(title string, items []list.Item) *listModel {
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = title
	return &listModel{list: l}
}

func (m listModel) Init() tea.Cmd {
	return nil
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.choice = i.title
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m listModel) View() string {
	return docStyle.Render(m.list.View())
}

func SelectFromList[T any](title string, items []T, printLineFunc func(T) string) (string, error) {
	listItems := make([]list.Item, len(items))
	for i, it := range items {
		listItems[i] = list.Item(item{title: printLineFunc(it)})
	}

	model := NewList(title, listItems)
	p := tea.NewProgram(model)
	m, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running program: %v", err)
	}

	if m.(listModel).quitting {
		return "", fmt.Errorf("selection cancelled")
	}

	return m.(listModel).choice, nil
}

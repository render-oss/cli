package testhelper

import "github.com/charmbracelet/bubbletea"

type FakeDimensionModel struct {
	Value  string
	Width  int
	Height int
}

func (f *FakeDimensionModel) Init() tea.Cmd {
	return nil
}

func (f *FakeDimensionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return f, nil
}

func (f *FakeDimensionModel) View() string {
	return f.Value
}

func (f *FakeDimensionModel) SetWidth(width int) {
	f.Width = width
}

func (f *FakeDimensionModel) SetHeight(height int) {
	f.Height = height
}

package tui

import tea "github.com/charmbracelet/bubbletea"

type LoadDataMsg[T any] struct {
	Data T
}

type LoadingDataMsg tea.Cmd
type DoneLoadingDataMsg struct{}

// TypedCmd is a wrapper around tea.Cmd that allows us to specify the type of
// data that the command will return. Since tea.Cmd is just a function that returns
// a message, it can be used to return any type of data. This wrapper allows us to
// have a more type-safe way of dealing with commands that return specific types of
// data.
//
// Only the wrapper should be used to create a TypedCmd. And the function inside the
// type should never be executed directly. Instead, we should return the `tea.Cmd`
// from an Update or Init function on a `tea.Model`.
type TypedCmd[D any] tea.Cmd

func (c TypedCmd[D]) Unwrap() tea.Cmd {
	return (tea.Cmd)(c)
}

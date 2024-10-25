package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type FormAction[T any] struct {
	action    func(T) tea.Cmd
	onSubmit  TypedCmd[T]
	submitted bool
}

func NewFormAction[T any](
	action func(T) tea.Cmd,
	onSubmit TypedCmd[T],
) FormAction[T] {
	return FormAction[T]{
		action:   action,
		onSubmit: onSubmit,
	}
}

func (fa *FormAction[T]) Init() tea.Cmd {
	return fa.onSubmit.Unwrap()
}

func (fa *FormAction[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LoadDataMsg[T]:
		cmd := fa.action(msg.Data)
		return fa, cmd
	}

	return fa, nil
}

func (fa *FormAction[T]) View() string {
	return "Loading..."
}

type FormWithAction[T any] struct {
	done       bool
	formAction FormAction[T]
	huhForm    *huh.Form
}

func NewFormWithAction[T any](action FormAction[T], form *huh.Form) *FormWithAction[T] {
	return &FormWithAction[T]{
		formAction: action,
		huhForm:    form,
	}
}

func (df *FormWithAction[T]) Init() tea.Cmd {
	df.done = false
	return df.huhForm.Init()
}

func (df *FormWithAction[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			df.done = true
			return df, df.formAction.Init()
		}
	}

	var cmd tea.Cmd
	if df.done {
		_, cmd = df.formAction.Update(msg)
	} else {
		_, cmd = df.huhForm.Update(msg)
	}

	return df, cmd
}

func (df *FormWithAction[T]) View() string {
	if df.done {
		return df.formAction.View()
	}

	return df.huhForm.View()
}

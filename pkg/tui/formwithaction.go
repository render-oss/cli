package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type FormAction[T any] struct {
	action   func(T) tea.Cmd
	onSubmit TypedCmd[T]
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

type FormWithAction[T any] struct {
	formAction FormAction[T]
	buildForm  func() *huh.Form
	huhForm    *huh.Form
}

// NewFormWithAction wires the form's natural submit flow to the action's
// onSubmit cmd via huh.Form.SubmitCmd. Once huh transitions to
// StateCompleted, its View() returns "" (because f.quitting is true). We
// also send tea.ClearScreen so the bubble tea renderer fully wipes the
// previous (taller) form render, rather than relying on its diff logic to
// erase every line.
//
// buildForm is a factory invoked on every Init(). huh provides no public
// API to reset a completed form, so a fresh instance is required each
// time the model is re-entered (e.g. when the user navigates back via
// Esc after a successful submission).
func NewFormWithAction[T any](action FormAction[T], buildForm func() *huh.Form) *FormWithAction[T] {
	return &FormWithAction[T]{
		formAction: action,
		buildForm:  buildForm,
	}
}

func (df *FormWithAction[T]) Init() tea.Cmd {
	df.huhForm = df.buildForm()
	df.huhForm.SubmitCmd = tea.Sequence(tea.ClearScreen, df.formAction.onSubmit.Unwrap())
	return df.huhForm.Init()
}

func (df *FormWithAction[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case StackSizeMsg:
		// Forward dimensions to huh so it can scroll when the form is
		// taller than the terminal. Without this, fields below the fold
		// are unreachable.
		df.huhForm = df.huhForm.WithWidth(msg.Width).WithHeight(msg.Height)
		return df, nil
	case LoadDataMsg[T]:
		// Loading finished; dispatch the post-load action.
		return df, df.formAction.action(msg.Data)
	}

	f, cmd := df.huhForm.Update(msg)
	if hf, ok := f.(*huh.Form); ok {
		df.huhForm = hf
	}
	return df, cmd
}

func (df *FormWithAction[T]) View() string {
	if df.huhForm == nil {
		return ""
	}
	return df.huhForm.View()
}

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type FormAction struct {
	model     tea.Model
	modelFunc func() (tea.Model, error)
	onSubmit  func() tea.Cmd
	submitted bool
}

func NewFormAction(
	modelFunc func() (tea.Model, error),
	onSubmit func() tea.Cmd,
) FormAction {
	return FormAction{
		modelFunc: modelFunc,
		onSubmit:  onSubmit,
	}
}

func (fa *FormAction) Init() tea.Cmd {
	return fa.onSubmit()
}

func (fa *FormAction) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if fa.model == nil {
		var err error
		fa.model, err = fa.modelFunc()
		if err != nil {
			return fa, func() tea.Msg {
				return ErrorMsg{Err: err}
			}
		}
		return fa.model, fa.model.Init()
	}

	var cmd tea.Cmd
	fa.model, cmd = fa.model.Update(msg)
	return fa, cmd
}

func (fa *FormAction) View() string {
	if fa.model == nil {
		return "Loading..."
	}

	return fa.model.View()
}

type FormWithAction struct {
	done       bool
	formAction FormAction
	huhForm    *huh.Form
}

func NewFormWithAction(action FormAction, form *huh.Form) *FormWithAction {
	return &FormWithAction{
		formAction: action,
		huhForm:    form,
	}
}

func (df *FormWithAction) Init() tea.Cmd {
	return df.huhForm.Init()
}

func (df *FormWithAction) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			return &df.formAction, df.formAction.Init()
		}
	}

	_, cmd := df.huhForm.Update(msg)
	return df, cmd
}

func (df *FormWithAction) View() string {
	if df.done {
		return df.formAction.View()
	}

	return df.huhForm.View()
}

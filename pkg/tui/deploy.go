package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type FormAction struct {
	model        tea.Model
	logModelFunc func() (tea.Model, error)
	onSubmit     func() tea.Cmd
	submitted    bool
	logModel     tea.Model
}

func NewFormAction(
	form *huh.Form,
	logModelFunc func() (tea.Model, error),
	onSubmit func() tea.Cmd,
) FormAction {
	return FormAction{
		model:        form,
		logModelFunc: logModelFunc,
		onSubmit:     onSubmit,
	}
}

func (fa *FormAction) Init() tea.Cmd {
	return fa.model.Init()
}

func (fa *FormAction) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !fa.submitted {
		fa.submitted = true
		return fa, fa.onSubmit()
	}

	if fa.logModel == nil {
		var err error
		fa.logModel, err = fa.logModelFunc()
		if err != nil {
			return fa, func() tea.Msg {
				return ErrorMsg{Err: err}
			}
		}
		return fa.logModel, fa.logModel.Init()
	}

	var cmd tea.Cmd
	fa.logModel, cmd = fa.logModel.Update(msg)
	return fa, cmd
}

func (fa *FormAction) View() string {
	if !fa.submitted {
		return fa.model.View()
	}
	if fa.logModel == nil {
		return "Loading logs..."
	}
	return fa.logModel.View()
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
	if df.done {
		return df.formAction.Update(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if !df.done {
				df.done = true
				return df, nil
			}
		case tea.KeyCtrlC, tea.KeyEsc:
			return df, tea.Quit
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

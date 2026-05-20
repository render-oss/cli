package workflows

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	workflows "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/tui"
)

var inputHintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4d4d4d")).Italic(true)

type TaskRunView struct {
	formAction *tui.FormWithAction[*workflows.TaskRun]
}

func NewTaskRunView(
	ctx context.Context,
	workflowLoader *WorkflowLoader,
	input *TaskRunInput,
	cobraCmd *cobra.Command,
	action func(j *workflows.TaskRun) tea.Cmd,
) *TaskRunView {
	fields, values := command.HuhFormFields(cobraCmd, input)

	return &TaskRunView{
		formAction: tui.NewFormWithAction(
			tui.NewFormAction(
				action,
				func() tea.Msg {
					var createTaskRunInput TaskRunInput
					err := command.StructFromFormValues(values, &createTaskRunInput)
					if err != nil {
						return tui.ErrorMsg{Err: err}
					}
					return command.LoadCmd(ctx, workflowLoader.CreateTaskRun, createTaskRunInput)()
				},
			),
			func() *huh.Form { return huh.NewForm(huh.NewGroup(fields...)) },
		),
	}
}

func (v *TaskRunView) Init() tea.Cmd {
	return v.formAction.Init()
}

func (v *TaskRunView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v.formAction.Update(msg)
}

func (v *TaskRunView) View() string {
	base := v.formAction.View()
	if base == "" {
		return base
	}
	hint := inputHintStyle.Render("ctrl+j  newline  ·  enter  confirm  ·  ctrl+e  open in editor")
	return base + "\n" + hint
}

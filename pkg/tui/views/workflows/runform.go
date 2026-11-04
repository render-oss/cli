package workflows

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/tui"
)

var inputHintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4d4d4d")).Italic(true)

// FlowRunInput is the form input for starting a flow run interactively.
type FlowRunInput struct {
	Input string `cli:"input" cli-lines:"15" cli-ext:"json" validate:"required"`
}

// RunView is a form that starts a task or flow run. Its fields come from
// cobraCmd's flags that match In's cli tags; submitting it calls startRun with
// the filled-in input, then action with the started run.
type RunView[In any, Run any] struct {
	formAction *tui.FormWithAction[Run]
}

func NewRunView[In any, Run any](
	ctx context.Context,
	startRun func(context.Context, In) (Run, error),
	input *In,
	cobraCmd *cobra.Command,
	action func(run Run) tea.Cmd,
) *RunView[In, Run] {
	fields, values := command.HuhFormFields(cobraCmd, input)

	return &RunView[In, Run]{
		formAction: tui.NewFormWithAction(
			tui.NewFormAction(
				action,
				func() tea.Msg {
					var startInput In
					err := command.StructFromFormValues(values, &startInput)
					if err != nil {
						return tui.ErrorMsg{Err: err}
					}
					return command.LoadCmd(ctx, startRun, startInput)()
				},
			),
			func() *huh.Form { return huh.NewForm(huh.NewGroup(fields...)) },
		),
	}
}

func (v *RunView[In, Run]) Init() tea.Cmd {
	return v.formAction.Init()
}

func (v *RunView[In, Run]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v.formAction.Update(msg)
}

func (v *RunView[In, Run]) View() string {
	base := v.formAction.View()
	if base == "" {
		return base
	}
	hint := inputHintStyle.Render("ctrl+j  newline  ·  enter  confirm  ·  ctrl+e  open in editor")
	return base + "\n" + hint
}

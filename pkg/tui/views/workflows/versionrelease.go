package workflows

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/tui"
)

const versionTimeout = time.Hour
const versionReleaseTimeout = time.Minute

type VersionReleaseView struct {
	formAction     *tui.FormWithAction[*wfclient.WorkflowVersion]
	workflowLoader *WorkflowLoader

	ctx    context.Context
	input  *VersionReleaseInput
	logCmd func(v *wfclient.WorkflowVersion) tea.Cmd
}

func NewVersionReleaseView(ctx context.Context, workflowLoader *WorkflowLoader, input *VersionReleaseInput, logCmd func(wfv *wfclient.WorkflowVersion) tea.Cmd) *VersionReleaseView {
	return &VersionReleaseView{
		ctx:            ctx,
		input:          input,
		workflowLoader: workflowLoader,
		logCmd:         logCmd,
	}
}

func (v *VersionReleaseView) setupForm() tea.Cmd {
	var inputs []huh.Field
	if v.input.CommitID == nil {
		v.input.CommitID = pointers.From("")
	}

	inputs = append(inputs, huh.NewInput().
		Title("Commit ID").
		Placeholder("Enter commit ID (optional)").
		Value(v.input.CommitID))

	versionForm := huh.NewForm(huh.NewGroup(inputs...))

	action := tui.NewFormAction(
		v.logCmd,
		command.WrapInConfirm(command.LoadCmd(v.ctx, v.workflowLoader.ReleaseVersion, *v.input), v.workflowLoader.VersionReleaseConfirm(v.ctx, *v.input)),
	)

	v.formAction = tui.NewFormWithAction(action, versionForm)

	return v.formAction.Init()
}

func (v *VersionReleaseView) Init() tea.Cmd {
	return v.setupForm()
}

func (v *VersionReleaseView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if v.formAction == nil {
		return nil, nil
	}

	return v.formAction.Update(msg)
}

func (v *VersionReleaseView) View() string {
	if v.formAction == nil {
		return ""
	}
	return v.formAction.View()
}

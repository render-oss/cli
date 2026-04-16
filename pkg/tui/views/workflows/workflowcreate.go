package workflows

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/workflow"
)

type WorkflowCreateInput struct {
	Name              *string `cli:"name"`
	Repo              *string `cli:"repo"`
	Branch            *string `cli:"branch"`
	Runtime           *string `cli:"runtime"`
	BuildCommand      *string `cli:"build-command"`
	RunCommand        *string `cli:"run-command"`
	Region            *string `cli:"region"`
	RootDir           *string `cli:"root-directory"`
	AutoDeployTrigger *string `cli:"auto-deploy-trigger"`
}

func (w WorkflowCreateInput) Validate(interactive bool) error {
	if interactive {
		return nil
	}

	if w.Name == nil || *w.Name == "" {
		return fmt.Errorf("--name is required")
	}
	if w.Repo == nil || *w.Repo == "" {
		return fmt.Errorf("--repo is required")
	}
	if w.Runtime == nil || *w.Runtime == "" {
		return fmt.Errorf("--runtime is required")
	}
	if w.BuildCommand == nil || *w.BuildCommand == "" {
		return fmt.Errorf("--build-command is required")
	}
	if w.RunCommand == nil || *w.RunCommand == "" {
		return fmt.Errorf("--run-command is required")
	}

	return nil
}

func CreateWorkflow(ctx context.Context, input WorkflowCreateInput) (*wfclient.Workflow, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	workflowRepo := workflow.NewRepo(c)

	ownerID, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	region := wfclient.Oregon
	if input.Region != nil && *input.Region != "" {
		region = wfclient.Region(*input.Region)
	}

	autoDeployVal := wfclient.Commit
	if input.AutoDeployTrigger != nil && *input.AutoDeployTrigger != "" {
		autoDeployVal = wfclient.AutoDeployTrigger(*input.AutoDeployTrigger)
	}
	autoDeployTrigger := &autoDeployVal

	body := client.CreateWorkflowJSONRequestBody{
		Name:              pointers.ValueOrDefault(input.Name, ""),
		OwnerId:           ownerID,
		Region:            region,
		RunCommand:        pointers.ValueOrDefault(input.RunCommand, ""),
		AutoDeployTrigger: autoDeployTrigger,
		BuildConfig: wfclient.BuildConfig{
			Repo:         pointers.ValueOrDefault(input.Repo, ""),
			BuildCommand: pointers.ValueOrDefault(input.BuildCommand, ""),
			Runtime:      wfclient.Runtime(pointers.ValueOrDefault(input.Runtime, "")),
			Branch:       input.Branch,
			RootDir:      input.RootDir,
		},
	}

	return workflowRepo.CreateWorkflow(ctx, body)
}

type WorkflowCreateView struct {
	formAction *tui.FormWithAction[*wfclient.Workflow]
}

func NewWorkflowCreateView(
	ctx context.Context,
	input *WorkflowCreateInput,
	cobraCmd *cobra.Command,
	createWorkflow func(ctx context.Context, input WorkflowCreateInput) (*wfclient.Workflow, error),
	action func(w *wfclient.Workflow) tea.Cmd,
) *WorkflowCreateView {
	fields, values := command.HuhFormFields(cobraCmd, input)

	return &WorkflowCreateView{
		formAction: tui.NewFormWithAction(
			tui.NewFormAction(
				action,
				func() tea.Msg {
					var createInput WorkflowCreateInput
					err := command.StructFromFormValues(values, &createInput)
					if err != nil {
						return tui.ErrorMsg{Err: err}
					}
					return command.LoadCmd(ctx, createWorkflow, createInput)()
				},
			),
			huh.NewForm(huh.NewGroup(fields...)),
		),
	}
}

func (v *WorkflowCreateView) Init() tea.Cmd {
	return v.formAction.Init()
}

func (v *WorkflowCreateView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v.formAction.Update(msg)
}

func (v *WorkflowCreateView) View() string {
	return v.formAction.View()
}

package workflows

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	envvar "github.com/render-oss/cli/pkg/client/envvar"
	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/types"
	"github.com/render-oss/cli/pkg/workflow"
)

type WorkflowCreateInput struct {
	Name              *string `cli:"name" validate:"required"`
	Repo              *string `cli:"repo" validate:"required"`
	Branch            *string `cli:"branch"`
	Runtime           *string `cli:"runtime" validate:"required"`
	BuildCommand      *string `cli:"build-command" validate:"required"`
	RunCommand        *string `cli:"run-command" validate:"required"`
	Region            *string `cli:"region"`
	RootDir           *string `cli:"root-directory"`
	AutoDeployTrigger *string `cli:"auto-deploy-trigger"`
	// EnvVars is a flat list of "KEY=VALUE" pairs. The cobra layer
	// (cmd/workflowcreate.go) is responsible for loading any --env-file
	// contents and prepending them here before invoking CreateWorkflow, so
	// the view doesn't need to know about env files. Inline --env-var entries
	// must be appended after file-derived ones so they override on duplicate
	// keys (resolveEnvVars merges via a map; later writes win).
	EnvVars 		  []string `cli:"env-var"`
}

func CreateWorkflow(ctx context.Context, input WorkflowCreateInput) (*wfclient.Workflow, error) {
	envVars, err := resolveEnvVars(input)
	if err != nil {
		return nil, err
	}

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
		EnvVars: pointers.FromArray(envVars),
	}

	return workflowRepo.CreateWorkflow(ctx, body)
}

// resolveEnvVars parses input.EnvVars (a flat list of "KEY=VALUE" pairs) into
// the envvar.EnvVarInput union form expected by the workflow create API. Later
// entries override earlier ones on duplicate keys.
func resolveEnvVars(input WorkflowCreateInput) ([]envvar.EnvVarInput, error) {
	if len(input.EnvVars) == 0 {
		return nil, nil
	}

	merged := make(map[string]string, len(input.EnvVars))
	for _, raw := range input.EnvVars {
		ev, err := types.ParseEnvVar(raw)
		if err != nil {
			return nil, err
		}
		merged[ev.Key] = ev.Value
	}

	out := make([]envvar.EnvVarInput, 0, len(merged))
	for k, v := range merged {
		var envVarInput envvar.EnvVarInput
		if err := envVarInput.FromEnvVarKeyValue(envvar.EnvVarKeyValue{Key: k, Value: v}); err != nil {
			return nil, fmt.Errorf("failed to encode env var %q: %w", k, err)
		}
		out = append(out, envVarInput)
	}
	return out, nil
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
			func() *huh.Form { return huh.NewForm(huh.NewGroup(fields...)) },
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

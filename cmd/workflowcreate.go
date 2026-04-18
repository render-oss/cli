package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	wfclient "github.com/render-oss/cli/v2/pkg/client/workflows"
	"github.com/render-oss/cli/v2/pkg/command"
	"github.com/render-oss/cli/v2/pkg/dependencies"
	"github.com/render-oss/cli/v2/pkg/pointers"
	"github.com/render-oss/cli/v2/pkg/text"
	"github.com/render-oss/cli/v2/pkg/tui/flows"
	"github.com/render-oss/cli/v2/pkg/tui/views"
	wfviews "github.com/render-oss/cli/v2/pkg/tui/views/workflows"
	"github.com/render-oss/cli/v2/pkg/types"
)

var workflowRuntimeValues = []string{
	string(wfclient.Node),
	string(wfclient.Python),
	string(wfclient.Go),
	string(wfclient.Ruby),
	string(wfclient.Elixir),
}

var workflowAutoDeployTriggerValues = []string{
	string(wfclient.Commit),
	string(wfclient.Off),
	string(wfclient.ChecksPass),
}

var WorkflowCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new workflow service",
	Args:  cobra.NoArgs,
	Long: `Create a new workflow service on Render.

In interactive mode, a form guides you through the required fields.
In non-interactive mode, provide all required config with flags.

Examples:
  render workflows create
  render workflows create --name my-workflow --repo https://github.com/org/repo --build-command "npm install" --runtime node --run-command "npm start" --region oregon -o json
`,
}

func init() {
	WorkflowCreateCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input wfviews.WorkflowCreateInput

		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return fmt.Errorf("failed to parse input: %w", err)
		}

		if nonInteractive, err := command.NonInteractive(cmd, func() (*wfclient.Workflow, error) {
			return wfviews.CreateWorkflow(cmd.Context(), input)
		}, func(w *wfclient.Workflow) string {
			return text.FormatStringF("Created workflow %s (%s)", w.Name, w.Id)
		}); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		interactiveWorkflowCreate(cmd, &input)
		return nil
	}

	WorkflowCreateCmd.Flags().String("name", "", "Workflow name. Required in non-interactive mode.")
	WorkflowCreateCmd.Flags().String("repo", "", "Git repository URL. Required in non-interactive mode.")
	WorkflowCreateCmd.Flags().String("branch", "", "Git branch (optional)")
	runtimeFlag := command.NewEnumInput(workflowRuntimeValues, false)
	WorkflowCreateCmd.Flags().Var(runtimeFlag, "runtime", "Runtime (node, python, go, ruby, elixir). Required in non-interactive mode.")
	WorkflowCreateCmd.Flags().String("build-command", "", "Build command. Required in non-interactive mode.")
	WorkflowCreateCmd.Flags().String("run-command", "", "Command to run the workflow. Required in non-interactive mode.")
	regionFlag := command.NewEnumInput(types.RegionValues(), false)
	WorkflowCreateCmd.Flags().Var(regionFlag, "region", "Deployment region (default: oregon)")
	WorkflowCreateCmd.Flags().String("root-directory", "", "Root directory in the repository (optional)")
	autoDeployFlag := command.NewEnumInput(workflowAutoDeployTriggerValues, false)
	WorkflowCreateCmd.Flags().Var(autoDeployFlag, "auto-deploy-trigger", "Autodeploy behavior (commit, off, checksPass; default: commit)")
}

func interactiveWorkflowCreate(cmd *cobra.Command, input *wfviews.WorkflowCreateInput) tea.Cmd {
	ctx := cmd.Context()
	deps := dependencies.GetFromContext(ctx)

	// Set defaults for enum fields so the interactive form cursor starts
	// on the right value instead of the first item alphabetically.
	if input.Region == nil {
		input.Region = pointers.From("oregon")
	}
	if input.AutoDeployTrigger == nil {
		input.AutoDeployTrigger = pointers.From(string(wfclient.Commit))
	}
	return command.AddToStackFunc(
		ctx,
		cmd,
		"Create Workflow",
		input,
		wfviews.NewWorkflowCreateView(ctx, input, WorkflowCreateCmd, wfviews.CreateWorkflow, func(w *wfclient.Workflow) tea.Cmd {
			if w.AutoDeployTrigger == nil || *w.AutoDeployTrigger == wfclient.Off {
				return nil
			}
			return flows.NewLogFlow(deps).LogsFlow(ctx, views.LogInput{
				ResourceIDs: []string{w.Id},
				Tail:        true,
			})
		}),
	)
}

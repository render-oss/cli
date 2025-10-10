package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/flows"
	workflowviews "github.com/render-oss/cli/pkg/tui/views/workflows"
)

func NewTaskListCmd(deps flows.WorkflowDeps) *cobra.Command {
	taskListCmd := &cobra.Command{
		Use:   "list [workflowVersionID]",
		Short: "List tasks in a workflow version",
		Long: `List all tasks defined in a workflow version.

Tasks are user-defined functions registered with the Render workflow SDK. Each time you
release a workflow service, Render creates a new workflow version and registers all tasks
it finds in that version.

In interactive mode, you will be prompted to select a workflow version if not provided.

Local Development:
When using the --local flag, you don't need to provide a workflow version ID. Instead,
the command connects to your local dev server (default port 8120) to list tasks from
your running workflow service. Start the dev server with 'render ea tasks dev' first.

Examples:
  render ea tasks list wfv-1234
  render ea tasks list --local
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, local, err := getLocalDeps(cmd, deps)
			if err != nil {
				return fmt.Errorf("failed to get local deps: %w", err)
			}

			var input workflowviews.TaskListInput
			err = command.ParseCommand(cmd, args, &input)
			if err != nil {
				return fmt.Errorf("failed to parse command: %w", err)
			}

			if nonInteractive, err := command.NonInteractive(cmd, func() ([]*wfclient.Task, error) {
				_, res, err := deps.WorkflowLoader().LoadTaskList(cmd.Context(), input, "")
				return res, err
			}, text.TaskTable); err != nil {
				return err
			} else if nonInteractive {
				return nil
			}

			flows.NewWorkflow(deps, flows.NewLogFlow(deps, flows.WithLocal(local)), local).TaskListFlow(cmd.Context(), &input)

			return nil
		},
	}

	return taskListCmd
}

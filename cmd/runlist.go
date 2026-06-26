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

func NewRunListCmd(deps flows.WorkflowDeps) *cobra.Command {
	var runListCmd = &cobra.Command{
		Use:   "list [taskID]",
		Short: "List all execution runs for a specific task",
		Long: `List all execution runs for a specific task.

A task run represents a single execution of a task with specific input parameters. This command shows the history of all runs for a given task.

You can specify the task by its workflow slug and task name (e.g., my-workflow/my-task), either as a positional argument or with --task.

In interactive mode, you will be prompted to select a task if not provided.`,
		Example: `  # List task runs by task ID
  render workflows tasks runs list --task tsk-1234

  # List task runs by task slug
  render workflows tasks runs list --task my-workflow/my-task

  # List task runs by passing the task as a positional argument
  render workflows tasks runs list my-workflow/my-task

  # List task runs from local workflow development server
  render workflows tasks runs list --local --task my-task`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, local, err := getLocalDeps(cmd, deps)
			if err != nil {
				return fmt.Errorf("failed to get local deps: %w", err)
			}

			var input workflowviews.TaskRunListInput
			input.Local = local
			err = command.ParseCommand(cmd, args, &input)
			if err != nil {
				return fmt.Errorf("failed to parse command: %w", err)
			}

			if input.TaskSlug == "" {
				if taskFlag, ferr := cmd.Flags().GetString("task"); ferr == nil && taskFlag != "" {
					input.TaskSlug = taskFlag
				}
			}

			if nonInteractive, err := command.NonInteractive(cmd, func() ([]*wfclient.TaskRun, error) {
				_, res, err := deps.WorkflowLoader().LoadTaskRunList(cmd.Context(), input, "")
				return res, err
			}, text.TaskRunTable); err != nil {
				return err
			} else if nonInteractive {
				return nil
			}

			flows.NewWorkflow(deps, flows.NewLogFlow(deps, flows.WithLocal(local)), local).RunListFlow(cmd.Context(), &input)
			return nil
		},
	}

	runListCmd.Flags().String("task", "", "ID or slug of the task whose runs to list (alternative to the positional argument)")
	setFlagPlaceholder(runListCmd.Flags(), "task", "TASK")

	return runListCmd
}

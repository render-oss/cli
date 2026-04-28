package cmd

import (
	"fmt"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/tui/flows"
	workflowviews "github.com/render-oss/cli/pkg/tui/views/workflows"
	"github.com/spf13/cobra"
)

func NewRunCancelCmd(deps flows.WorkflowDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "cancel [taskRunID]",
		Short: "Cancel a running task run",
		Long: `Cancel an in-progress task run.

Use --local to cancel a task run in the local workflow development server.`,
		Example: `  # Cancel a remote task run
  render workflows runs cancel trn-abc123

  # Cancel a task run in the local dev server
  render workflows runs cancel --local trn-xyz789`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, _, err := getLocalDeps(cmd, deps)
			if err != nil {
				return fmt.Errorf("failed to get local deps: %w", err)
			}

			var input workflowviews.TaskRunTargetInput
			if err := command.ParseCommand(cmd, args, &input); err != nil {
				return fmt.Errorf("failed to parse command: %w", err)
			}

			if err := deps.WorkflowLoader().CancelTaskRun(cmd.Context(), input.TaskRunID); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cancelled task run %s\n", input.TaskRunID)
			return nil
		},
	}
}

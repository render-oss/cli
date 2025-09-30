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

func NewTaskRunListCmd(deps flows.WorkflowDeps) *cobra.Command {
	var taskRunListCmd = &cobra.Command{
		Use:   "runs [taskID]",
		Short: "List task runs for a task",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, local, err := getLocalDeps(cmd, deps)
			if err != nil {
				return fmt.Errorf("failed to get local deps: %w", err)
			}

			var input workflowviews.TaskRunListInput
			err = command.ParseCommand(cmd, args, &input)
			if err != nil {
				return fmt.Errorf("failed to parse command: %w", err)
			}

			if nonInteractive, err := command.NonInteractive(cmd, func() ([]*wfclient.TaskRun, error) {
				_, res, err := deps.WorkflowLoader().LoadTaskRunList(cmd.Context(), input, "")
				return res, err
			}, text.TaskRunTable); err != nil {
				return err
			} else if nonInteractive {
				return nil
			}

			flows.NewWorkflow(deps, flows.NewLogFlow(deps, flows.WithLocal(local)), local).TaskRunListFlow(cmd.Context(), &input)
			return nil
		},
	}

	return taskRunListCmd
}

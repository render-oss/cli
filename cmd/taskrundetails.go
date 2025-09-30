package cmd

import (
	"fmt"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/flows"
	workflowviews "github.com/render-oss/cli/pkg/tui/views/workflows"
	"github.com/spf13/cobra"
)

func NewTaskRunDetailsCmd(deps flows.WorkflowDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "details [taskRunID]",
		Short: "Get details for a task run",
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, local, err := getLocalDeps(cmd, deps)
			if err != nil {
				return fmt.Errorf("failed to get local deps: %w", err)
			}

			var input workflowviews.TaskRunDetailsInput
			err = command.ParseCommand(cmd, args, &input)
			if err != nil {
				return fmt.Errorf("failed to parse command: %w", err)
			}

			if nonInteractive, err := command.NonInteractive(cmd, func() (*wfclient.TaskRunDetails, error) {
				res, err := deps.WorkflowLoader().LoadTaskRunDetails(cmd.Context(), &input)
				return res, err
			}, text.TaskRunDetails); err != nil {
				return err
			} else if nonInteractive {
				return nil
			}

			flows.NewWorkflow(deps, flows.NewLogFlow(deps, flows.WithLocal(local)), local).TaskRunDetailsFlow(cmd.Context(), &input)

			return nil
		},
	}
}

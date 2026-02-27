package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/resource"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/flows"
	workflowviews "github.com/render-oss/cli/pkg/tui/views/workflows"
	"github.com/render-oss/cli/pkg/workflow"
)

func NewWorkflowListCmd(deps flows.WorkflowDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List workflows",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var input workflowviews.WorkflowInput
			err := command.ParseCommand(cmd, args, &input)
			if err != nil {
				return fmt.Errorf("failed to parse command: %w", err)
			}

			if nonInteractive, err := command.NonInteractive(cmd, func() ([]*workflow.Model, error) {
				return deps.WorkflowLoader().ListWorkflows(cmd.Context(), input)
			}, func(models []*workflow.Model) string {
				resources := make([]resource.Resource, len(models))
				for i, m := range models {
					resources[i] = m
				}
				return text.ResourceTable(resources)
			}); err != nil {
				return err
			} else if nonInteractive {
				return nil
			}

			flows.NewWorkflow(deps, flows.NewLogFlow(deps), false).WorkflowListPaletteFlow(cmd.Context(), &input)

			return nil
		},
	}
}

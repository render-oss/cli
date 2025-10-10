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

func NewVersionListCmd(deps flows.WorkflowDeps) *cobra.Command {
	var versionListCmd = &cobra.Command{
		Use:   "list [workflowID]",
		Short: "List versions of a workflow",
		Long: `List all versions of a workflow service.

Each time you release a workflow service, Render creates a new workflow version. A version
represents a specific snapshot of your workflow service code and its registered tasks at
the time of release.

This command displays all versions for a workflow, showing:
  • Version ID
  • Creation timestamp
  • Associated tasks

In interactive mode, you will be prompted to select a workflow if not provided.

Examples:
  render ea versions list wf-1234
  render ea versions list my-workflow-slug
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var input workflowviews.VersionListInput
			err := command.ParseCommand(cmd, args, &input)
			if err != nil {
				return fmt.Errorf("failed to parse command: %w", err)
			}

			if nonInteractive, err := command.NonInteractive(cmd, func() ([]*wfclient.WorkflowVersion, error) {
				_, res, err := deps.WorkflowLoader().LoadVersionList(cmd.Context(), input, "")
				return res, err
			}, text.VersionTable); err != nil {
				return err
			} else if nonInteractive {
				return nil
			}
			flows.NewWorkflow(deps, flows.NewLogFlow(deps), false).VersionList(cmd.Context(), &input)
			return nil
		},
	}

	return versionListCmd
}

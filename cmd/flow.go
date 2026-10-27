package cmd

import (
	"github.com/spf13/cobra"

	workflows "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/flowrun"
)

// The flows commands stay hidden until durable workflows are generally
// available.
func newFlowsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "flows",
		Short:  "Inspect durable workflow runs",
		Hidden: true,
	}
	cmd.AddCommand(children...)
	return cmd
}

func newFlowRunsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "Inspect flow runs",
	}
	cmd.AddCommand(children...)
	return cmd
}

func newFlowRunShowCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "show <flowRunID>",
		Short:        "Show detailed information about a flow run",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Display a flow run's status, input, and result.

If the run failed because a replay didn't match its history, the output names
the step and what changed. If it failed on a limit, the output names the limit
and the run's value.`,
		Example: `  # Show details for a flow run
  render workflows flows runs show frn-1234

  # JSON output
  render workflows flows runs show frn-1234 --output json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		flowRunID := args[0]
		_, err := command.NonInteractive(cmd, func() (*workflows.FlowRunDetails, error) {
			return deps.FlowRunRepo().GetFlowRun(cmd.Context(), flowRunID)
		}, flowrun.Details)
		return err
	}

	return cmd
}

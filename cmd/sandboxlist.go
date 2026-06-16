package cmd

import (
	"github.com/spf13/cobra"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxListInput struct {
	Status []string `cli:"status"`
	All    bool     `cli:"all"`
}

func newSandboxListCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sandboxes",
		Long: `List sandboxes in your workspace.

By default, terminated sandboxes are excluded. Use --all to include them, or --status to filter by specific statuses.

Examples:
  render ea sandbox list
  render ea sandbox list --all
  render ea sandbox list --status=running
  render ea sandbox list --status=running --status=creating
  render ea sandbox list -o json
`,
	}

	cmd.Flags().StringArray("status", nil, "Filter by status (repeatable: creating, running, errored, terminated)")
	cmd.Flags().Bool("all", false, "Include terminated sandboxes")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxListInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() ([]*sandboxclient.Sandbox, error) {
			return deps.SandboxService().List(cmd.Context(), input.Status, input.All)
		}, text.SandboxTable)
		return err
	}

	return cmd
}

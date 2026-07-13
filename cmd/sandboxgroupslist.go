package cmd

import (
	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

func newSandboxGroupsListCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List sandbox groups in the active workspace",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		Long: `List sandbox groups in your workspace.

Early Access guarantees at most one default group per workspace, so this
command typically prints a single row.`,
		Example: `  # List sandbox groups in the active workspace
  render ea sandbox-groups list

  # Output as JSON
  render ea sandbox-groups list -o json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		_, err := command.NonInteractive(cmd, func() ([]*sandboxesclient.SandboxGroup, error) {
			return deps.SandboxGroupService().List(cmd.Context())
		}, text.SandboxGroupTable)
		return err
	}

	return cmd
}

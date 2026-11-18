package cmd

import (
	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxGroupsEnvironmentSetInput struct {
	SandboxGroupID      string `cli:"arg:0"`
	EnvironmentIDOrName string `cli:"arg:1"`
}

func newSandboxGroupsEnvironmentSetCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "set <sandboxGroupId> <environment>",
		Short:        "Put a sandbox group in a project environment",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		Long: `Put a sandbox group in a project environment, or move it to another one.

The environment is an ID or a name in the active workspace. Moving a group
into or out of a protected environment requires an admin.`,
		Example: `  # Put a group in an environment, or move it to another one
  render sandbox-groups environment set sbg-abc123 production

  # Output as JSON
  render sandbox-groups environment set sbg-abc123 evm-abc123def456 -o json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxGroupsEnvironmentSetInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() (*sandboxesclient.SandboxGroup, error) {
			return deps.SandboxGroupService().SetEnvironment(
				cmd.Context(), input.SandboxGroupID, input.EnvironmentIDOrName)
		}, text.SandboxGroupDetail)
		return err
	}
	return cmd
}

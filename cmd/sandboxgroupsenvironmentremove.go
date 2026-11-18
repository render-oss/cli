package cmd

import (
	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxGroupsEnvironmentRemoveInput struct {
	SandboxGroupID string `cli:"arg:0"`
}

func newSandboxGroupsEnvironmentRemoveCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "remove <sandboxGroupId>",
		Short:        "Take a sandbox group out of its project environment",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Take a sandbox group out of its project environment.

Running this on a group that isn't in an environment makes no changes. Moving
a group out of a protected environment requires an admin.`,
		Example: `  # Take a group out of its environment
  render sandbox-groups environment remove sbg-abc123

  # Output as JSON
  render sandbox-groups environment remove sbg-abc123 -o json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxGroupsEnvironmentRemoveInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() (*sandboxesclient.SandboxGroup, error) {
			return deps.SandboxGroupService().ClearEnvironment(cmd.Context(), input.SandboxGroupID)
		}, text.SandboxGroupDetail)
		return err
	}
	return cmd
}

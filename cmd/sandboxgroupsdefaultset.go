package cmd

import (
	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxGroupsDefaultSetInput struct {
	SandboxGroupID string `cli:"arg:0"`
}

func newSandboxGroupsDefaultSetCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "set <sandboxGroupId>",
		Short:        "Make a sandbox group the workspace's default",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Make a sandbox group the default for its workspace.

The previous default stops being the default. Running this on the current
default makes no changes. A suspended group cannot be made the default.`,
		Example: `  # Make a group the default
  render sandbox-groups default set sbg-abc123

  # Output as JSON
  render sandbox-groups default set sbg-abc123 -o json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxGroupsDefaultSetInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() (*sandboxesclient.SandboxGroup, error) {
			return deps.SandboxGroupService().SetDefault(cmd.Context(), input.SandboxGroupID)
		}, text.SandboxGroupDetail)
		return err
	}
	return cmd
}

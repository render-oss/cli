package cmd

import (
	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxBuildsGetInput struct {
	BuildID string `cli:"arg:0"`
	Group   string `cli:"group"`
}

func newSandboxBuildsGetCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get <buildId>",
		Short:        "Get a sandbox build",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Get one sandbox build by ID.

Uses the active workspace's default sandbox group unless --group is provided.
A build that belongs to another sandbox group is reported as not found.`,
		Example: `  # Get a build
  render sandbox-groups builds get sbb-abc123

  # Get a build in a specific group
  render sandbox-groups builds get sbb-abc123 --group sbg-abc123

  # JSON output
  render sandbox-groups builds get sbb-abc123 --output json`,
	}

	cmd.Flags().String("group", "", "Sandbox group the build belongs to (defaults to the active workspace's default group)")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxBuildsGetInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() (*sandboxesclient.SandboxBuild, error) {
			return deps.SandboxBuildService().Get(cmd.Context(), input.Group, input.BuildID)
		}, text.SandboxBuildDetail)
		return err
	}

	return cmd
}

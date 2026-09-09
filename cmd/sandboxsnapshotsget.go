package cmd

import (
	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxSnapshotsGetInput struct {
	SnapshotID string `cli:"arg:0"`
	Group      string `cli:"group"`
}

func newSandboxSnapshotsGetCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get <snapshotId>",
		Short:        "Get a sandbox snapshot",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Get one sandbox snapshot by ID.

Uses the active workspace's default sandbox group unless --group is provided.
A deleted or expired snapshot is reported as not found.`,
		Example: `  # Get a snapshot
  render ea sandboxes snapshots get snp-abc123

  # Get a snapshot in a specific group
  render ea sandboxes snapshots get snp-abc123 --group sbg-abc123

  # JSON output
  render ea sandboxes snapshots get snp-abc123 --output json`,
	}

	cmd.Flags().String("group", "", "Sandbox group the snapshot belongs to (defaults to the active workspace's default group)")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxSnapshotsGetInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() (*sandboxesclient.SandboxSnapshot, error) {
			return deps.SandboxSnapshotService().Get(cmd.Context(), input.Group, input.SnapshotID)
		}, text.SandboxSnapshotDetail)
		return err
	}

	return cmd
}

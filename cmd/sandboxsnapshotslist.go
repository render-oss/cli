package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandboxsnapshot"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxSnapshotsListInput struct {
	Group  string   `cli:"group"`
	Status []string `cli:"status"`
}

func (i *SandboxSnapshotsListInput) Validate(_ bool) error {
	for _, status := range i.Status {
		if !sandboxesclient.SandboxSnapshotStatus(status).Valid() {
			return fmt.Errorf("invalid status %q: use %s", status, strings.Join(sandboxSnapshotStatusNames(), ", "))
		}
	}
	return nil
}

func sandboxSnapshotStatusNames() []string {
	return []string{
		string(sandboxesclient.SandboxSnapshotStatusCreating),
		string(sandboxesclient.SandboxSnapshotStatusAvailable),
		string(sandboxesclient.SandboxSnapshotStatusFailed),
	}
}

func newSandboxSnapshotsListCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List sandbox snapshots",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		Long: `List the snapshots in a sandbox group, newest first.

Uses the active workspace's default sandbox group unless --group is provided.
Deleted and expired snapshots are not listed.`,
		Example: `  # List every snapshot in the default sandbox group
  render ea sandboxes snapshots list

  # List snapshots in a specific group
  render ea sandboxes snapshots list --group sbg-abc123

  # Only snapshots that are ready to start a sandbox from
  render ea sandboxes snapshots list --status available

  # JSON output
  render ea sandboxes snapshots list --output json`,
	}

	cmd.Flags().String("group", "", "Sandbox group to list (defaults to the active workspace's default group)")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")
	cmd.Flags().StringArray("status", nil, "Filter by status (repeatable: "+strings.Join(sandboxSnapshotStatusNames(), ", ")+")")
	setFlagPlaceholder(cmd.Flags(), "status", "STATUS")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxSnapshotsListInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() ([]*sandboxesclient.SandboxSnapshot, error) {
			return deps.SandboxSnapshotService().List(cmd.Context(), sandboxsnapshot.ListInput{
				SandboxGroupID: input.Group,
				Statuses:       input.Status,
			})
		}, text.SandboxSnapshotTable)
		return err
	}

	return cmd
}

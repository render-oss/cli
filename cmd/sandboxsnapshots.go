package cmd

import (
	"github.com/spf13/cobra"
)

func newSandboxSnapshotsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshots",
		Short: "Manage sandbox snapshots",
		Long: `Manage snapshots of sandboxes in your Render workspace.

A snapshot captures a running sandbox so a new sandbox can be restored from it with "render sandboxes create --snapshot-id". A filesystem snapshot captures the writable filesystem and restores onto any plan. A runtime snapshot also captures memory and CPU state and restores only onto the plan of the source sandbox.

Snapshots belong to the sandbox group of their source sandbox. Getting, listing, or deleting a snapshot uses the active workspace's default group unless --group is provided.`,
		Example: `  # Snapshot a running sandbox
  render sandboxes snapshots create sbx-abc123

  # List the snapshots in the default sandbox group
  render sandboxes snapshots list

  # List the snapshots in a specific sandbox group
  render sandboxes snapshots list --group sbg-abc123

  # Get one snapshot
  render sandboxes snapshots get snp-abc123

  # Delete a snapshot
  render sandboxes snapshots delete snp-abc123 --confirm

  # Delete a snapshot in a specific group
  render sandboxes snapshots delete snp-abc123 --group sbg-abc123 --confirm`,
	}
	cmd.AddCommand(children...)
	return cmd
}

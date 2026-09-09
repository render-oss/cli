package cmd

import (
	"github.com/spf13/cobra"
)

func newSandboxSnapshotsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshots",
		Short: "Manage sandbox snapshots",
		Long: `Manage snapshots of sandboxes in your Render workspace.

A snapshot captures a running sandbox so a new sandbox can start from it. A
filesystem snapshot captures the writable filesystem and restores onto any
plan. A runtime snapshot also captures memory and CPU state and restores only
onto the plan of the source sandbox.

Snapshots belong to the sandbox group of their source sandbox.`,
		Example: `  # Snapshot a running sandbox
  render ea sandboxes snapshots create sbx-abc123`,
	}
	cmd.AddCommand(children...)
	return cmd
}

package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func newSandboxCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sandboxes",
		Short:   "Manage sandboxes",
		GroupID: GroupCore.ID,
		Long: `Manage sandboxes for your Render workspace.

Sandboxes are ephemeral compute environments for running code, agents, and experiments.

Every sandbox belongs to a sandbox group, which scopes it to a region. Manage
groups with "render sandbox-groups".

Snapshot a sandbox with "render sandboxes snapshots create" and restore new
sandboxes from it with "render sandboxes create --snapshot-id".

Examples:
  render sandboxes create
  render sandboxes create --plan=2c-4g --region=oregon
  render sandboxes create --snapshot-id snp-abc123
  render sandboxes copy ./main.py sbx-abc123:/app/main.py
  render sandboxes exec sbx-abc123 -- echo hello
  render sandboxes snapshots create sbx-abc123
  render sandboxes snapshots list --group sbg-abc123
  render sandboxes stop sbx-abc123 --confirm
`,
	}
	cmd.AddCommand(children...)
	return cmd
}

// waitContext turns Ctrl-C and SIGTERM into a cancelled context, so an
// interrupted --wait still prints what it created.
func waitContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
}

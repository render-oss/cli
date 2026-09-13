package cmd

import (
	"github.com/spf13/cobra"
)

func newSandboxCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandboxes",
		Short: "Manage sandboxes",
		Long: `Manage sandboxes for your Render workspace.

Sandboxes are ephemeral compute environments for running code, agents, and experiments.

Every sandbox belongs to a sandbox group, which scopes it to a region. Manage
groups with "render ea sandbox-groups".

Snapshot a sandbox with "render ea sandboxes snapshots create" and restore new
sandboxes from it with "render ea sandboxes create --snapshot-id".

Examples:
  render ea sandboxes create
  render ea sandboxes create --plan=standard --region=oregon
  render ea sandboxes create --snapshot-id snp-abc123
  render ea sandboxes copy ./main.py sbx-abc123:/app/main.py
  render ea sandboxes exec sbx-abc123 -- echo hello
  render ea sandboxes snapshots create sbx-abc123
  render ea sandboxes snapshots list --group sbg-abc123
  render ea sandboxes stop sbx-abc123 --confirm
`,
	}
	cmd.AddCommand(children...)
	return cmd
}

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

Examples:
  render ea sandboxes create
  render ea sandboxes create --plan=standard --region=oregon
  render ea sandboxes copy ./main.py sbx-abc123:/app/main.py
  render ea sandboxes exec sbx-abc123 -- echo hello
  render ea sandboxes list
  render ea sandboxes list --all
  render ea sandboxes stop sbx-abc123 --confirm
`,
	}
	cmd.AddCommand(children...)
	return cmd
}

package cmd

import (
	"github.com/spf13/cobra"
)

func newSandboxGroupsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox-groups",
		Short: "Manage sandbox groups",
		Long: `Manage sandbox groups for your Render workspace.

Sandbox groups scope a workspace's sandboxes to a region and (optionally) an
environment. Early Access guarantees at most one default group per workspace;
Beta will add multi-group support.

Examples:
  render ea sandbox-groups list
  render ea sandbox-groups list -o json
`,
	}
	cmd.AddCommand(children...)
	return cmd
}

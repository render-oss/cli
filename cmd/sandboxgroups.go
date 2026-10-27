package cmd

import (
	"github.com/spf13/cobra"
)

func newSandboxGroupsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sandbox-groups",
		Short:   "Manage sandbox groups",
		GroupID: GroupCore.ID,
		Long: `Manage sandbox groups for your Render workspace.

Sandbox groups scope a workspace's sandboxes to a region and (optionally) an
environment. The workspace's first group is its default.

Images that sandboxes start from belong to a sandbox group; see
"render sandbox-groups images".

Examples:
  render sandbox-groups list
  render sandbox-groups list -o json
  render sandbox-groups create --name agents
  render sandbox-groups images import python:3.13 --name python
`,
	}
	cmd.AddCommand(children...)
	return cmd
}

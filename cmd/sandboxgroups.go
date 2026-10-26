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
environment.

Builds of sandbox base images belong to a sandbox group; see
"render ea sandbox-groups builds".

Examples:
  render ea sandbox-groups list
  render ea sandbox-groups list -o json
  render ea sandbox-groups builds create python:3.13 --name python
`,
	}
	cmd.AddCommand(children...)
	return cmd
}

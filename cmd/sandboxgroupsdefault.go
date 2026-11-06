package cmd

import (
	"github.com/spf13/cobra"
)

func newSandboxGroupsDefaultCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "default",
		Short: "Manage the workspace's default sandbox group",
		Long: `Manage the workspace's default sandbox group.

The default group is where "render sandboxes create" places sandboxes unless
--group is given. A workspace has exactly one default group.`,
	}
	cmd.AddCommand(children...)
	return cmd
}

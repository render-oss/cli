package cmd

import (
	"github.com/spf13/cobra"
)

func newSandboxGroupsEnvironmentCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "environment",
		Short: "Manage the project environment a sandbox group is in",
		Long: `Manage the project environment a sandbox group is in.

A sandbox group is in at most one environment. A group that isn't in an
environment isn't in a project.`,
	}
	cmd.AddCommand(children...)
	return cmd
}

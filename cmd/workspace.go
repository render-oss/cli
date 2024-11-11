package cmd

import (
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Short:   "Manage the CLI's active workspace",
	Long: 	`Manage the CLI's active workspace. All CLI commands run against the active workspace.`,
	GroupID: GroupAuth.ID,
}

func init() {
	rootCmd.AddCommand(workspaceCmd)
}

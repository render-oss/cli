package cmd

import "github.com/spf13/cobra"

var WorkflowsCmd = &cobra.Command{
	Use:   "workflows",
	Short: "Manage workflows",
	Long: `Manage workflow services for the active workspace.

List workflows, browse versions and tasks, start task runs, and trigger releases.`,
	GroupID: GroupCore.ID,
}

func init() {
	rootCmd.AddCommand(WorkflowsCmd)
}

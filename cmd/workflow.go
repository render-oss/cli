package cmd

import "github.com/spf13/cobra"

var workflowCmd = &cobra.Command{
	Use:    "workflow",
	Hidden: true,
	Short:  "Manage workflows",
}

var workflowListCmd = &cobra.Command{
	Use:    "list",
	Hidden: true,
	Short:  "List workflows",
}

func init() {
	EarlyAccessCmd.AddCommand(workflowCmd)
	workflowCmd.AddCommand(workflowListCmd)
}

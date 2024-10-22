package cmd

import (
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspaces",
}

func init() {
	rootCmd.AddCommand(workspaceCmd)
}

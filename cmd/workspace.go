package cmd

import (
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Short:   "Manage CLI targeted workspace",
	GroupID: GroupAuth.ID,
}

func init() {
	rootCmd.AddCommand(workspaceCmd)
}

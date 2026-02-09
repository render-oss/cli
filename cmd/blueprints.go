package cmd

import (
	"github.com/spf13/cobra"
)

var blueprintsCmd = &cobra.Command{
	Use:     "blueprints",
	Short:   "Manage blueprints",
	Long:    `Manage blueprint files (render.yaml) including validation.`,
	GroupID: GroupManagement.ID,
}

func init() {
	rootCmd.AddCommand(blueprintsCmd)
}

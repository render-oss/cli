package cmd

import (
	"github.com/spf13/cobra"
)

var blueprintsCmd = &cobra.Command{
	Use:     "blueprints",
	Short:   "Manage Blueprints",
	Long:    `Manage Blueprint files (render.yaml) including validation.`,
	GroupID: GroupManagement.ID,
}

func init() {
	rootCmd.AddCommand(blueprintsCmd)
}

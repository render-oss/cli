package cmd

import (
	"github.com/spf13/cobra"
)

var blueprintsCmd = &cobra.Command{
	Use:     "blueprints",
	Short:   "Manage Blueprints, which define your infrastructure as code",
	Long:    `Manage Blueprint files (render.yaml) including validation.`,
	GroupID: GroupManagement.ID,
	Example: `  # Validate a blueprint file
  render blueprints validate ./render.yaml`,
}

func init() {
	rootCmd.AddCommand(blueprintsCmd)
}

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

func newEarlyAccessBlueprintsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blueprints",
		Short: "Manage Blueprints",
		Long: `Manage your Render infrastructure as code with Blueprints.

A Blueprint defines services, databases, and environment groups in a YAML file
(render.yaml by default) stored in a Git repository. Syncing a Blueprint applies
that file's configuration to its resources.

Learn more: https://render.com/docs/infrastructure-as-code`,
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(children...)
	return cmd
}

package cmd

import (
	"github.com/spf13/cobra"

	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/command"
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

// exitForPlanningOutcome returns the exit status for a planning outcome. Failed
// planning exits 1 without an error message, so the caller must show the user
// the planning outcome. Other outcomes exit 0, including PlanningOutcome values
// the API added after this CLI was generated.
func exitForPlanningOutcome(cmd *cobra.Command, outcome bptypes.PlanningOutcome) error {
	//exhaustive:enforce
	switch outcome {
	case bptypes.PlanningOutcomeFailed:
		cmd.SilenceErrors = true
		return command.NewExitError(1, nil)
	case bptypes.PlanningOutcomePlanned, bptypes.PlanningOutcomeNoChanges:
		return nil
	default:
		return nil
	}
}

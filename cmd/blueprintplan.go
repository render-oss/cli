package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/blueprint"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

func newBlueprintPlanCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "plan <blueprintId>",
		Short:        "Plan a sync of a Blueprint",
		Args:         cobra.ExactArgs(1),
		Hidden:       true,
		SilenceUsage: true,
		Long: `Plan changes from the latest Blueprint file on the Blueprint's branch without
applying them, by creating a new sync. Render reads the remote repository, so
push local changes first if you want them to be considered in planning.

Pass a Blueprint ID (exs-...), not a sync ID. Planning does not apply changes,
even when auto-sync is enabled for the Blueprint. Approve the planned sync with
"render ea blueprints approve". A planned sync that is not approved within 24
hours is deleted. If no changes are needed, no sync is created.

Some plans need manually-supplied environment variables (env vars with
sync: false in the Blueprint file). When a successful plan needs them, this
command creates a file for them at
` + blueprint.ManualEnvVarsFileHelpPath + `. Values start as empty strings;
you can edit the file before approving the sync. When a new sync is planned,
any earlier manual env var file for the Blueprint is replaced, or removed if the
new plan needs no manual values. Set RENDER_CLI_CONFIG_DIR to use a different
config directory.`,
		Example: `  # Plan a sync from the latest pushed Blueprint file
  render ea blueprints plan exs-d0k8jv3e4t2s73a1b2c0

  # Output the planning result as JSON
  render ea blueprints plan exs-d0k8jv3e4t2s73a1b2c0 --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			command.DefaultFormatNonInteractive(cmd)
			var input blueprint.PlanInput
			if err := command.ParseCommand(cmd, args, &input); err != nil {
				return err
			}
			var result *bptypes.CreateBlueprintSyncResponse
			var fileErr error
			_, err := command.NonInteractive(cmd,
				func() (blueprint.PlanOut, error) {
					var err error
					result, err = deps.BlueprintService().Plan(cmd.Context(), input.BlueprintID)
					if err != nil {
						return blueprint.PlanOut{}, err
					}
					// A new planned sync makes any manual env var file from an earlier plan
					// stale, because the Blueprint file may now require different env vars.
					// Writing the new file is not enough on its own: if every sync: false
					// env var was removed, no new file is written, and approve would read
					// the old one.
					if result.PlanningOutcome == bptypes.PlanningOutcomePlanned {
						if err := blueprint.RemoveManualEnvVarsFile(input.BlueprintID); err != nil {
							// Returning the error here would skip printing the planned sync, so keep
							// it for RunE to return after the result is printed.
							fileErr = fmt.Errorf("%w. Delete the earlier manual environment variables file before running \"render ea blueprints approve\", or approve will send its outdated values", err)
							return blueprint.PlanOut{Data: result}, nil
						}
					}
					filePath, err := blueprint.PrepareManualEnvVarsFile(input.BlueprintID, result.PlanningOutcome, result.Sync)
					if err != nil {
						// Returning the error here would skip printing the planned sync, so keep
						// it for RunE to return after the result is printed.
						fileErr = err
						return blueprint.PlanOut{Data: result}, nil
					}
					return blueprint.NewPlanOut(input.BlueprintID, result, filePath), nil
				},
				text.BlueprintPlan,
			)
			if err != nil {
				return err
			}
			if fileErr != nil {
				return fileErr
			}
			if result == nil {
				return errors.New("plan blueprint: command completed without a planning result")
			}
			return exitForPlanningOutcome(cmd, result.PlanningOutcome)
		},
	}
	return cmd
}

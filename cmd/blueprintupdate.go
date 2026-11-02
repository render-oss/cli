package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/blueprint"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

func newBlueprintUpdateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "update <blueprintId>",
		Short:        "Update a Blueprint's settings",
		Args:         cobra.ExactArgs(1),
		Hidden:       true,
		SilenceUsage: true,
		Long: `Update a Blueprint's name, branch, Blueprint file path, or automatic sync
setting. Only the flags you pass are changed. The repository cannot be changed.

Pass a Blueprint ID (exs-...), not a sync ID. The branch must contain a
Blueprint file at the configured path.

Changing --branch or --path also plans a sync from the new Blueprint file, as
"render ea blueprints plan" does. Approve it with "render ea blueprints approve".
Changing only --name or --auto-sync does not plan a sync. If the update succeeds
but planning fails, the update still stands; fix the errors and run
"render ea blueprints plan".

Like "render ea blueprints plan", a successful plan that needs manual values
writes a file for manually-supplied environment variables at
` + blueprint.ManualEnvVarsFileHelpPath + `. A newly planned sync replaces
any earlier file for the Blueprint, or removes it if the new plan needs no
manual values.`,
		Example: `  # Read the Blueprint file from a different branch and plan a sync from it
  render ea blueprints update exs-d0k8jv3e4t2s73a1b2c0 --branch release

  # Read the Blueprint file from a different file path on the remote repository (relative to the repository root)
  render ea blueprints update exs-d0k8jv3e4t2s73a1b2c0 --path infra/render.yaml

  # Disable automatic syncs
  render ea blueprints update exs-d0k8jv3e4t2s73a1b2c0 --auto-sync=false`,
		RunE: func(cmd *cobra.Command, args []string) error {
			command.DefaultFormatNonInteractive(cmd)
			var input blueprint.UpdateInput
			if err := command.ParseCommand(cmd, args, &input); err != nil {
				return err
			}
			var result *blueprint.UpdateResult
			var fileErr error
			_, err := command.NonInteractive(cmd,
				func() (blueprint.UpdateOut, error) {
					var err error
					result, err = deps.BlueprintService().Update(cmd.Context(), input)
					if err != nil {
						return blueprint.UpdateOut{}, err
					}
					if result.PlanningOutcome == nil {
						return blueprint.NewUpdateOut(result, ""), nil
					}
					filePath, err := blueprint.ReplaceManualEnvVarsFile(input.BlueprintID, *result.PlanningOutcome, result.Sync)
					if err != nil {
						// Print the updated Blueprint and planning result before reporting a
						// local file error.
						fileErr = err
						return blueprint.UpdateOut{Data: result, Meta: blueprint.UpdateOutMeta{SyncRequested: true}}, nil
					}
					return blueprint.NewUpdateOut(result, filePath), nil
				},
				text.BlueprintUpdate,
			)
			if err != nil {
				return err
			}
			if fileErr != nil {
				return fileErr
			}
			if result == nil {
				return errors.New("update blueprint: command completed without an update result")
			}
			if result.PlanningOutcome == nil {
				return nil
			}
			return exitForPlanningOutcome(cmd, *result.PlanningOutcome)
		},
	}

	cmd.Flags().String("name", "", "Set the Blueprint name")
	cmd.Flags().String("branch", "", "Set the branch to read the Blueprint file from")
	cmd.Flags().String("path", "", "Set the Blueprint file path relative to the repository root")
	cmd.Flags().Bool("auto-sync", false, "Enable automatic syncs of future pushes (pass --auto-sync=false to disable)")
	setAllFlagPlaceholders(cmd, map[string]string{
		"name":   "NAME",
		"branch": "BRANCH",
		"path":   "PATH",
	})
	return cmd
}

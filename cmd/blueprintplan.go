package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/blueprint"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

func newBlueprintPlanCmd(_ *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "plan <blueprintId>",
		Short:        "Plan a sync of a Blueprint",
		Args:         cobra.ExactArgs(1),
		Hidden:       true,
		SilenceUsage: true,
		Long: `Plan changes from the latest Blueprint file on the Blueprint's branch without
applying them, by creating a new Sync. Render reads the remote repository, so push local changes first if you want them to be considered in planning.

Pass a Blueprint ID (exs-...), not a sync ID. Planning does not apply changes.
Approve the planned sync with "render ea blueprints approve". A planned Sync that
is not approved within 24 hours is deleted. If no changes are needed, no Sync
is created.

Some plans need manually-supplied environment variables (env vars with
sync: false in the Blueprint file). When a plan needs them, this command creates
a file for them at
` + blueprint.ManualEnvVarsFileHelpPath + `. Values start as empty strings;
you can edit the file before approving the sync. Existing manual env var files
are replaced. Set RENDER_CLI_CONFIG_DIR to use a different config directory.

This command is under development. Execution currently returns an error without
planning a sync.`,
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
			_, err := command.NonInteractive(cmd,
				func() (blueprint.PlanOut, error) {
					return planBlueprint(input)
				},
				blueprintPlanText,
			)
			return err
		},
	}
	return cmd
}

func planBlueprint(_ blueprint.PlanInput) (blueprint.PlanOut, error) {
	return blueprint.PlanOut{}, errors.New("blueprint planning is not implemented yet")
}

func blueprintPlanText(_ blueprint.PlanOut) string {
	return "not implemented"
}

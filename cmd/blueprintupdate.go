package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/blueprint"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

func newBlueprintUpdateCmd(_ *dependencies.Dependencies) *cobra.Command {
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
Changing only --name or --auto-sync does not plan a sync.

This command is under development. Execution currently returns an error without
updating a Blueprint.`,
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
			_, err := command.NonInteractive(cmd,
				func() (blueprint.UpdateOut, error) {
					return updateBlueprint(input)
				},
				blueprintUpdateText,
			)
			return err
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

func updateBlueprint(_ blueprint.UpdateInput) (blueprint.UpdateOut, error) {
	return blueprint.UpdateOut{}, errors.New("blueprint update is not implemented yet")
}

func blueprintUpdateText(_ blueprint.UpdateOut) string {
	return "not implemented"
}

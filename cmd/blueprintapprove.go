package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/blueprint"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

func newBlueprintApproveCmd(_ *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "approve <blueprintId>",
		Short:        "Approve a Blueprint's newest sync",
		Args:         cobra.ExactArgs(1),
		Hidden:       true,
		SilenceUsage: true,
		Long: `Approve the Blueprint's newest sync to start applying its planned changes.
The command does not wait for the sync to finish.

Pass a Blueprint ID (exs-...), not a sync ID. If the newest sync is already
pending or running, the command returns it unchanged.

Some plans need manually-supplied environment variables (env vars with
sync: false in the Blueprint file). The Blueprint file does not contain their
values, so you supply them when you approve. When a plan needs them,
"render ea blueprints create" and "render ea blueprints plan" write a file listing
each one with an empty value to
` + blueprint.ManualEnvVarsFileHelpPath + `. Fill in the values, then
approve. Values are sent as written, so any value you leave as an empty string
is set to an empty string.

By default, this command reads that file when it exists, and deletes it after
the sync is approved so the values do not stay on disk. Use
--manual-env-vars-file to read a different file, or pass - to read from stdin,
which lets you pipe values in without writing them to disk. A file passed with
--manual-env-vars-file is never deleted. The file is a JSON array of objects
with serviceType, serviceName, key, and value, not .env format. Set
RENDER_CLI_CONFIG_DIR to use a different config directory.

This command is under development. Execution currently returns an error without
approving a sync.`,
		Example: `  # Approve the newest sync, reading values from the file create or plan wrote, if any
  render ea blueprints approve exs-d0k8jv3e4t2s73a1b2c0

  # Read values from a different file
  render ea blueprints approve exs-d0k8jv3e4t2s73a1b2c0 --manual-env-vars-file prod-env-vars.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			command.DefaultFormatNonInteractive(cmd)
			var input blueprint.ApproveInput
			if err := command.ParseCommand(cmd, args, &input); err != nil {
				return err
			}
			_, err := command.NonInteractive(cmd,
				func() (blueprint.ApproveOut, error) {
					return approveBlueprint(input)
				},
				blueprintApproveText,
			)
			return err
		},
	}

	cmd.Flags().String("manual-env-vars-file", "", "Read manually-supplied environment variables from a JSON file, or - for stdin (defaults to "+blueprint.ManualEnvVarsFileHelpPath+")")
	_ = cmd.MarkFlagFilename("manual-env-vars-file", "json")
	setAllFlagPlaceholders(cmd, map[string]string{
		"manual-env-vars-file": "PATH",
	})
	return cmd
}

func approveBlueprint(_ blueprint.ApproveInput) (blueprint.ApproveOut, error) {
	return blueprint.ApproveOut{}, errors.New("blueprint approval is not implemented yet")
}

func blueprintApproveText(_ blueprint.ApproveOut) string {
	return ""
}

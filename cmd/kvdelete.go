package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/resolve"
	"github.com/render-oss/cli/pkg/text"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

var kvDeleteCmd = &cobra.Command{
	Use:          "delete <keyValueID|keyValueName>",
	Short:        "Delete a Key Value store instance",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	Long: `Delete a Key Value store instance on Render.

Without --confirm, this command previews what would be deleted and makes no
changes. Pass --confirm to actually delete the instance.

The positional argument accepts either a Key Value ID (red-...) or a name.
If the name matches more than one instance, narrow the search with
--environment <id|name>, or pass the Key Value ID directly.

Name lookup is scoped to your active workspace. If a name isn't found, switch
workspaces with 'render workspace set <name|ID>' and try again, or pass the
Key Value ID instead (which works across workspaces).`,
	Example: `  # Preview deletion (no changes made)
  render ea kv delete red-abc123def456ghi789jkl0

  # Delete by ID
  render ea kv delete red-abc123def456ghi789jkl0 --confirm

  # Delete by name
  render ea kv delete my-cache --confirm

  # Disambiguate a name that exists in multiple environments
  render ea kv delete my-cache --environment production --confirm

  # JSON output
  render ea kv delete red-abc123def456ghi789jkl0 --confirm --output json`,
}

func init() {
	kvDeleteCmd.Flags().String("environment", "",
		"Environment ID or name (optional). Narrows name lookup when the same Key Value name exists in multiple environments.")

	kvDeleteCmd.RunE = func(cmd *cobra.Command, args []string) error {
		// No interactive flow yet; collapse --output interactive (the default in a TTY)
		// onto text so the standard NonInteractive path handles every format.
		command.DefaultFormatNonInteractive(cmd)

		var input kvtypes.KeyValueDeleteInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		input = kvtypes.NormalizeDeleteInput(input)
		confirm := command.GetConfirmFromContext(cmd.Context())

		loadData := func() (*keyvalue.DeleteResult, error) {
			var env *client.Environment
			if input.EnvironmentIDOrName != nil {
				c, err := client.NewDefaultClient()
				if err != nil {
					return nil, err
				}
				env, err = resolve.New(c).ResolveEnvironment(cmd.Context(), *input.EnvironmentIDOrName)
				if err != nil {
					return nil, err
				}
			}
			kv, err := keyvalue.Resolve(cmd.Context(), input.IDOrName, nil, env)
			if err != nil {
				return nil, err
			}
			if confirm {
				if err := keyvalue.Delete(cmd.Context(), kv.Id); err != nil {
					return nil, err
				}
			}
			return &keyvalue.DeleteResult{KeyValue: kv, Deleted: confirm}, nil
		}

		_, err := command.NonInteractive(cmd, loadData, formatTextOutput)
		return err
	}

	kvCmd.AddCommand(kvDeleteCmd)
}

func formatTextOutput(r *keyvalue.DeleteResult) string {
	if r.Deleted {
		return "Deleted this Key Value:\n\n" + text.KeyValueDetail(r.KeyValue) + "\n"
	}
	return "This command would delete this Key Value:\n\n" +
		text.KeyValueDetail(r.KeyValue) +
		"\nRe-run with --confirm to proceed\n"
}

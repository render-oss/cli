package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/text"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

func newKVDeleteCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete <keyValueID|keyValueName>",
		Short:        "Delete a Render Key Value instance",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Delete a Render Key Value instance.

Without --confirm, this command previews what would be deleted and makes no
changes. Pass --confirm to actually delete the instance.

The positional argument accepts either a Key Value ID (red-...) or a name.
If the name matches more than one instance, narrow the search with
--environment <id|name>, or pass the Key Value ID directly.

Name lookup is scoped to your active workspace. If a name isn't found, switch
workspaces with 'render workspace set <name|ID>' and try again, or pass the
Key Value ID instead (which works across workspaces).`,
		Example: `  # Preview deletion (no changes made)
  render kv delete red-abc123def456ghi789jkl0

  # Delete by ID
  render kv delete red-abc123def456ghi789jkl0 --confirm

  # Delete by name
  render kv delete my-cache --confirm

  # Disambiguate a name that exists in multiple environments
  render kv delete my-cache --environment production --confirm

  # JSON output
  render kv delete red-abc123def456ghi789jkl0 --confirm --output json`,
	}

	cmd.Flags().String("environment", "",
		"Narrow lookup to an environment (ID or name, optional) when the same Key Value name exists in multiple environments.")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// No interactive flow yet; collapse --output interactive (the default in a TTY)
		// onto text so the standard NonInteractive path handles every format.
		command.DefaultFormatNonInteractive(cmd)

		var input kvtypes.KeyValueDeleteInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		input = kvtypes.NormalizeDeleteInput(input)
		confirm := command.GetConfirmFromContext(cmd.Context())

		loadData := func() (*keyvalue.DeleteOut, error) {
			resolved, err := deps.KeyValueService().Resolve(cmd.Context(), keyvalue.ResolveInput{
				IDOrName:            input.IDOrName,
				EnvironmentIDOrName: input.EnvironmentIDOrName,
			})
			if err != nil {
				return nil, err
			}
			out := keyvalue.DeleteOut{
				Data: keyvalue.NewKeyValueOut(resolved),
				Meta: keyvalue.DeleteOutMeta{
					Deleted: confirm,
				},
			}
			if confirm {
				if err := deps.KeyValueService().Delete(cmd.Context(), out.Data.ID); err != nil {
					return nil, err
				}
			} else {
				out.Meta.Message = "re-run with --confirm to delete"
			}
			return &out, nil
		}

		_, err := command.NonInteractive(cmd, loadData, formatDeleteTextOutput)
		return err
	}

	return cmd
}

func formatDeleteTextOutput(r *keyvalue.DeleteOut) string {
	if r.Meta.Deleted {
		return "Deleted this Key Value:\n\n" + text.KeyValueDetail(&r.Data) + "\n"
	}
	return "This command would delete this Key Value:\n\n" +
		text.KeyValueDetail(&r.Data) +
		"\nRe-run with --confirm to proceed\n"
}

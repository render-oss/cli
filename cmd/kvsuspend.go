package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/text"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

func newKVSuspendCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "suspend <keyValueID|keyValueName>",
		Short:        "Suspend a Key Value store instance",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Suspend a Key Value store instance on Render.

Without --confirm, this command previews what would be suspended and makes no
changes. Pass --confirm to actually suspend the instance.

The positional argument accepts either a Key Value ID (red-...) or a name.
If the name matches more than one instance, narrow the search with
--environment <id|name>, or pass the Key Value ID directly.

Name lookup is scoped to your active workspace. If a name isn't found, switch
workspaces with 'render workspace set <name|ID>' and try again, or pass the
Key Value ID instead (which works across workspaces).`,
		Example: `  # Preview suspension (no changes made)
  render ea kv suspend red-abc123def456ghi789jkl0

  # Suspend by ID
  render ea kv suspend red-abc123def456ghi789jkl0 --confirm

  # Suspend by name
  render ea kv suspend my-cache --confirm

  # Disambiguate a name that exists in multiple environments
  render ea kv suspend my-cache --environment production --confirm

  # JSON output
  render ea kv suspend red-abc123def456ghi789jkl0 --confirm --output json`,
	}

	cmd.Flags().String("environment", "",
		"Environment ID or name (optional). Narrows name lookup when the same Key Value name exists in multiple environments.")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input kvtypes.KeyValueSuspendInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		input = kvtypes.NormalizeSuspendInput(input)
		confirm := command.GetConfirmFromContext(cmd.Context())

		loadData := func() (*keyvalue.SuspendResult, error) {
			before, err := deps.KeyValueService().Resolve(cmd.Context(), keyvalue.ResolveInput{
				IDOrName:            input.IDOrName,
				EnvironmentIDOrName: input.EnvironmentIDOrName,
			})
			if err != nil {
				return nil, err
			}
			kv := before.KeyValue
			if !confirm {
				return &keyvalue.SuspendResult{KeyValue: kv, Suspended: false}, nil
			}
			if err := deps.KeyValueService().Suspend(cmd.Context(), kv.Id); err != nil {
				return nil, err
			}
			after, err := deps.KeyValueService().Resolve(cmd.Context(), keyvalue.ResolveInput{
				IDOrName: kv.Id,
			})
			if err != nil {
				return nil, err
			}
			return &keyvalue.SuspendResult{KeyValue: after.KeyValue, Suspended: true}, nil
		}

		_, err := command.NonInteractive(cmd, loadData, formatSuspendTextOutput)
		return err
	}

	return cmd
}

func formatSuspendTextOutput(r *keyvalue.SuspendResult) string {
	if r.Suspended {
		return "Suspended this Key Value:\n\n" + text.KeyValueAPIDetail(r.KeyValue) + "\n"
	}
	return "This command would suspend this Key Value:\n\n" +
		text.KeyValueAPIDetail(r.KeyValue) +
		"\n\nRe-run with --confirm to proceed\n"
}

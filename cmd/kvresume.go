package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/text"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

func newKVResumeCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "resume <keyValueID|keyValueName>",
		Short:        "Resume a suspended Render Key Value instance",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Resume a suspended Render Key Value instance.

The positional argument accepts either a Key Value ID (red-...) or a name.
If the name matches more than one instance, narrow the search with
--environment <id|name>, or pass the Key Value ID directly.

Name lookup is scoped to your active workspace. If a name isn't found, switch
workspaces with 'render workspace set <name|ID>' and try again, or pass the
Key Value ID instead (which works across workspaces).`,
		Example: `  # Resume by ID
  render kv resume red-abc123def456ghi789jkl0

  # Resume by name
  render kv resume my-cache

  # Disambiguate a name that exists in multiple environments
  render kv resume my-cache --environment production

  # JSON output
  render kv resume red-abc123def456ghi789jkl0 --output json`,
	}

	cmd.Flags().String("environment", "",
		"Narrow lookup to an environment (ID or name, optional) when the same Key Value name exists in multiple environments.")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input kvtypes.KeyValueResumeInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		input = kvtypes.NormalizeResumeInput(input)

		loadData := func() (*keyvalue.KeyValueResumeOut, error) {
			kv, err := deps.KeyValueService().Resolve(cmd.Context(), keyvalue.ResolveInput{
				IDOrName:            input.IDOrName,
				EnvironmentIDOrName: input.EnvironmentIDOrName,
			})
			if err != nil {
				return nil, err
			}
			if err := deps.KeyValueService().Resume(cmd.Context(), kv.KeyValue.Id); err != nil {
				return nil, err
			}
			kv, err = deps.KeyValueService().Resolve(cmd.Context(), keyvalue.ResolveInput{
				IDOrName: kv.KeyValue.Id,
			})
			if err != nil {
				return nil, err
			}
			out := keyvalue.NewKeyValueOut(kv)
			return &keyvalue.KeyValueResumeOut{Data: out}, nil
		}

		_, err := command.NonInteractive(cmd, loadData, formatResumeTextOutput)
		return err
	}

	return cmd
}

func formatResumeTextOutput(out *keyvalue.KeyValueResumeOut) string {
	return "Resumed this Key Value:\n\n" + text.KeyValueDetail(&out.Data) + "\n"
}

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/text"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

func newKVResumeCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "resume <keyValueID|keyValueName>",
		Short:        "Resume a suspended Key Value store instance",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Resume a suspended Key Value store instance on Render.

The positional argument accepts either a Key Value ID (red-...) or a name.
If the name matches more than one instance, narrow the search with
--environment <id|name>, or pass the Key Value ID directly.

Name lookup is scoped to your active workspace. If a name isn't found, switch
workspaces with 'render workspace set <name|ID>' and try again, or pass the
Key Value ID instead (which works across workspaces).`,
		Example: `  # Resume by ID
  render ea kv resume red-abc123def456ghi789jkl0

  # Resume by name
  render ea kv resume my-cache

  # Disambiguate a name that exists in multiple environments
  render ea kv resume my-cache --environment production

  # JSON output
  render ea kv resume red-abc123def456ghi789jkl0 --output json`,
	}

	cmd.Flags().String("environment", "",
		"Environment ID or name (optional). Narrows name lookup when the same Key Value name exists in multiple environments.")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input kvtypes.KeyValueResumeInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		input = kvtypes.NormalizeResumeInput(input)

		loadData := func() (*client.KeyValueDetail, error) {
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
			return kv.KeyValue, nil
		}

		_, err := command.NonInteractive(cmd, loadData, formatResumeTextOutput)
		return err
	}

	return cmd
}

func formatResumeTextOutput(kv *client.KeyValueDetail) string {
	return "Resumed this Key Value:\n\n" + text.KeyValueAPIDetail(kv) + "\n"
}

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/resolve"
	"github.com/render-oss/cli/pkg/text"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

func newKVResumeCmd(_ *dependencies.Dependencies) *cobra.Command {
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
			var project *client.Project
			var env *client.Environment
			if input.EnvironmentIDOrName != nil {
				c, err := client.NewDefaultClient()
				if err != nil {
					return nil, err
				}
				scope, err := resolve.NewFromClient(c).ResolveScopeInActiveWorkspace(cmd.Context(), resolve.ActiveWorkspaceScopeInput{
					EnvironmentIDOrName: input.EnvironmentIDOrName,
				})
				if err != nil {
					return nil, err
				}
				project = scope.Project
				env = scope.Environment
			}
			kv, err := keyvalue.Resolve(cmd.Context(), input.IDOrName, project, env)
			if err != nil {
				return nil, err
			}
			if err := keyvalue.Resume(cmd.Context(), kv.Id); err != nil {
				return nil, err
			}
			return keyvalue.Resolve(cmd.Context(), kv.Id, project, env)
		}

		_, err := command.NonInteractive(cmd, loadData, formatResumeTextOutput)
		return err
	}

	return cmd
}

func formatResumeTextOutput(kv *client.KeyValueDetail) string {
	return "Resumed this Key Value:\n\n" + text.KeyValueDetail(kv) + "\n"
}

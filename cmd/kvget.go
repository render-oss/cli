package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/text"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

func newKVGetCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get <keyValueID|keyValueName>",
		Short:        "Get details of a Render Key Value instance",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Get details and connection info for a Render Key Value instance.

The positional argument accepts either a Key Value ID (red-...) or a name.
If the name matches more than one instance, narrow the search with
--project <id|name>, --environment <id|name>, or pass the Key Value ID
directly.

Name lookup is scoped to your active workspace. If a name isn't found, switch
workspaces with 'render workspace set <name|ID>' and try again, or pass the
Key Value ID instead (which works across workspaces).`,
		Example: `  # Get by ID
  render kv get red-abc123def456ghi789jkl0

  # Get by name
  render kv get my-cache

  # Include connection strings (contains credentials)
  render kv get my-cache --include-sensitive-connection-info

  # Disambiguate by project
  render kv get my-cache --project my-project

  # Disambiguate a name that exists in multiple environments
  render kv get my-cache --environment production

  # JSON output
  render kv get red-abc123def456ghi789jkl0 --output json`,
	}

	cmd.Flags().String("project", "",
		"Narrow lookup to a project (ID or name, optional) within the active workspace.")
	cmd.Flags().String("environment", "",
		"Narrow lookup to an environment (ID or name, optional) when the same Key Value name exists in multiple environments.")
	cmd.Flags().Bool("include-sensitive-connection-info", false,
		"Include connection strings and credentials in the output")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input kvtypes.KeyValueGetInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		input = kvtypes.NormalizeGetInput(input)

		loadData := func() (*keyvalue.KeyValueGetOut, error) {
			resolved, err := deps.KeyValueService().Resolve(cmd.Context(), keyvalue.ResolveInput{
				IDOrName:            input.IDOrName,
				ProjectIDOrName:     input.ProjectIDOrName,
				EnvironmentIDOrName: input.EnvironmentIDOrName,
			})
			if err != nil {
				return nil, err
			}
			out := keyvalue.NewKeyValueOut(resolved)
			if input.IncludeSensitiveConnectionInfo {
				conn, err := deps.KeyValueService().GetConnectionInfo(cmd.Context(), out.ID)
				if err != nil {
					return nil, err
				}
				out.ConnectionInfo = conn
			}
			return &keyvalue.KeyValueGetOut{Data: out}, nil
		}

		_, err := command.NonInteractive(cmd, loadData, func(out *keyvalue.KeyValueGetOut) string {
			return text.KeyValueDetail(&out.Data) + "\n"
		})
		return err
	}

	return cmd
}

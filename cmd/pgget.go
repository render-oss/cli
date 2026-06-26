package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/text"
	pgtypes "github.com/render-oss/cli/pkg/types/postgres"
)

func newPgGetCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get <postgresID|postgresName>",
		Short:        "Get details of a Render Postgres database",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Get details and connection info for a Render Postgres database.

The positional argument accepts either a Postgres ID (dpg-...) or a name.
If the name matches more than one database, narrow the search with
--project <id|name>, --environment <id|name>, or pass the Postgres ID directly.

Name lookup is scoped to your active workspace. If a name isn't found, switch
workspaces with 'render workspace set <name|ID>' and try again, or pass the
Postgres ID instead (which works across workspaces).`,
		Example: `  # Get by ID
  render ea pg get dpg-abc123def456ghi789jkl0

  # Get by name
  render ea pg get my-db

  # Include connection strings (contains credentials)
  render ea pg get my-db --include-sensitive-connection-info

  # Disambiguate by project
  render ea pg get my-db --project my-project

  # Disambiguate a name that exists in multiple environments
  render ea pg get my-db --environment production

  # JSON output
  render ea pg get dpg-abc123def456ghi789jkl0 --output json`,
	}

	cmd.Flags().String("project", "",
		"Narrow lookup to a project (ID or name, optional) within the active workspace.")
	cmd.Flags().String("environment", "",
		"Narrow lookup to an environment (ID or name, optional) when the same Postgres database name exists in multiple environments.")
	cmd.Flags().Bool("include-sensitive-connection-info", false,
		"Include connection strings and credentials in the output")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input pgtypes.GetPostgresInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		input = pgtypes.NormalizeGetInput(input)

		loadData := func() (*postgres.GetOut, error) {
			resolved, err := deps.PostgresService().Resolve(cmd.Context(), postgres.ResolveInput{
				IDOrName:            input.IDOrName,
				ProjectIDOrName:     input.ProjectIDOrName,
				EnvironmentIDOrName: input.EnvironmentIDOrName,
			})
			if err != nil {
				return nil, err
			}
			out := postgres.NewPostgresGetOut(resolved)
			var conn *client.PostgresConnectionInfo
			if input.IncludeSensitiveConnectionInfo {
				conn, err = deps.PostgresService().GetConnectionInfo(cmd.Context(), out.Data.Id)
				if err != nil {
					return nil, err
				}
			}
			out.Data.ConnectionInfo = conn
			return &out, nil
		}

		_, err := command.NonInteractive(cmd, loadData, func(out *postgres.GetOut) string {
			return text.PostgresGetDetail(&out.Data) + "\n"
		})
		return err
	}

	return cmd
}

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

func newPgResumeCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "resume <postgresID|postgresName>",
		Short:        "Resume a suspended Postgres database",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Resume a suspended Postgres database on Render.

The positional argument accepts either a Postgres ID (dpg-...) or a name.
If the name matches more than one database, narrow the search with
--project <id|name>, --environment <id|name>, or pass the Postgres ID directly.

Name lookup is scoped to your active workspace. If a name isn't found, switch
workspaces with 'render workspace set <name|ID>' and try again, or pass the
Postgres ID instead (which works across workspaces).`,
		Example: `  # Resume by ID
  render ea pg resume dpg-abc123def456ghi789jkl0

  # Resume by name
  render ea pg resume my-db

  # Disambiguate a name that exists in multiple environments
  render ea pg resume my-db --environment production

  # Disambiguate a name that exists in multiple projects
  render ea pg resume my-db --project analytics

  # JSON output
  render ea pg resume dpg-abc123def456ghi789jkl0 --output json`,
	}

	cmd.Flags().String("project", "",
		"Project ID or name (optional). Narrows name lookup when the same Postgres database name exists in multiple projects.")
	cmd.Flags().String("environment", "",
		"Environment ID or name (optional). Narrows name lookup when the same Postgres database name exists in multiple environments.")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input pgtypes.ResumePostgresInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		input = pgtypes.NormalizeResumeInput(input)

		loadData := func() (*client.PostgresDetail, error) {
			pg, err := deps.PostgresService().Resolve(cmd.Context(), postgres.ResolveInput{
				IDOrName:            input.IDOrName,
				ProjectIDOrName:     input.ProjectIDOrName,
				EnvironmentIDOrName: input.EnvironmentIDOrName,
			})
			if err != nil {
				return nil, err
			}
			if err := deps.PostgresService().ResumePostgres(cmd.Context(), pg.Id); err != nil {
				return nil, err
			}
			return deps.PostgresService().Resolve(cmd.Context(), postgres.ResolveInput{IDOrName: pg.Id})
		}

		_, err := command.NonInteractive(cmd,
			loadData,
			pgResumeTextOutput,
		)
		return err
	}

	return cmd
}

func pgResumeTextOutput(pg *client.PostgresDetail) string {
	return "Resumed this Postgres database:\n\n" + text.PostgresDetail(pg) + "\n"
}

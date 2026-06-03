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

type pgSuspendResult struct {
	Postgres  *client.PostgresDetail `json:"postgres"`
	Suspended bool                   `json:"suspended"`
}

func newPgSuspendCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "suspend <postgresID|postgresName>",
		Short:        "Suspend a Postgres database",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Suspend a Postgres database on Render.

Without --confirm, this command previews what would be suspended and makes no
changes. Pass --confirm to actually suspend the database.

The positional argument accepts either a Postgres ID (dpg-...) or a name.
If the name matches more than one database, narrow the search with
--project <id|name>, --environment <id|name>, or pass the Postgres ID directly.

Name lookup is scoped to your active workspace. If a name isn't found, switch
workspaces with 'render workspace set <name|ID>' and try again, or pass the
Postgres ID instead (which works across workspaces).`,
		Example: `  # Preview suspension (no changes made)
  render ea pg suspend dpg-abc123def456ghi789jkl0

  # Suspend by ID
  render ea pg suspend dpg-abc123def456ghi789jkl0 --confirm

  # Suspend by name
  render ea pg suspend my-db --confirm

  # Disambiguate a name that exists in multiple environments
  render ea pg suspend my-db --environment production --confirm

  # Disambiguate a name that exists in multiple projects
  render ea pg suspend my-db --project analytics --confirm

  # JSON output
  render ea pg suspend dpg-abc123def456ghi789jkl0 --confirm --output json`,
	}

	cmd.Flags().String("project", "",
		"Project ID or name (optional). Narrows name lookup when the same Postgres database name exists in multiple projects.")
	cmd.Flags().String("environment", "",
		"Environment ID or name (optional). Narrows name lookup when the same Postgres database name exists in multiple environments.")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input pgtypes.SuspendPostgresInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		input = pgtypes.NormalizeSuspendInput(input)
		confirm := command.GetConfirmFromContext(cmd.Context())

		loadData := func() (*pgSuspendResult, error) {
			pg, err := deps.PostgresService().Resolve(cmd.Context(), postgres.ResolveInput{
				IDOrName:            input.IDOrName,
				ProjectIDOrName:     input.ProjectIDOrName,
				EnvironmentIDOrName: input.EnvironmentIDOrName,
			})
			if err != nil {
				return nil, err
			}
			if !confirm {
				return &pgSuspendResult{Postgres: pg, Suspended: false}, nil
			}
			if err := deps.PostgresService().SuspendPostgres(cmd.Context(), pg.Id); err != nil {
				return nil, err
			}
			post, err := deps.PostgresService().Resolve(cmd.Context(), postgres.ResolveInput{IDOrName: pg.Id})
			if err != nil {
				return nil, err
			}
			return &pgSuspendResult{Postgres: post, Suspended: true}, nil
		}

		_, err := command.NonInteractive(cmd,
			loadData,
			pgSuspendTextOutput,
		)
		return err
	}

	return cmd
}

func pgSuspendTextOutput(r *pgSuspendResult) string {
	if r.Suspended {
		return "Suspended this Postgres database:\n\n" + text.PostgresDetail(r.Postgres) + "\n"
	}
	return "This command would suspend this Postgres database:\n\n" +
		text.PostgresDetail(r.Postgres) +
		"\n\nRe-run with --confirm to proceed\n"
}

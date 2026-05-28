package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/types"
)

type pgDeleteInput struct {
	IDOrName            string  `cli:"arg:0"`
	ProjectIDOrName     *string `cli:"project"`
	EnvironmentIDOrName *string `cli:"environment"`
}

func normalizePGDeleteInput(input pgDeleteInput) pgDeleteInput {
	input.ProjectIDOrName = types.OptionalNonZeroString(input.ProjectIDOrName)
	input.EnvironmentIDOrName = types.OptionalNonZeroString(input.EnvironmentIDOrName)
	return input
}

type pgDeleteResult struct {
	Postgres *client.PostgresDetail `json:"postgres"`
	Deleted  bool                   `json:"deleted"`
}

func newPgDeleteCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete <postgresID|postgresName>",
		Short:        "Delete a Postgres database",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Delete a Postgres database on Render.

Without --confirm, this command previews what would be deleted and makes no
changes. Pass --confirm to actually delete the database.

The positional argument accepts either a Postgres ID (dpg-...) or a name.
If the name matches more than one database, narrow the search with
--project <id|name>, --environment <id|name>, or pass the Postgres ID directly.

Name lookup is scoped to your active workspace. If a name isn't found, switch
workspaces with 'render workspace set <name|ID>' and try again, or pass the
Postgres ID instead (which works across workspaces).`,
		Example: `  # Preview deletion (no changes made)
  render ea pg delete dpg-abc123def456ghi789jkl0

  # Delete by ID
  render ea pg delete dpg-abc123def456ghi789jkl0 --confirm

  # Delete by name
  render ea pg delete my-db --confirm

  # Disambiguate a name that exists in multiple environments
  render ea pg delete my-db --environment production --confirm

  # Disambiguate a name that exists in multiple projects
  render ea pg delete my-db --project analytics --confirm

  # JSON output
  render ea pg delete dpg-abc123def456ghi789jkl0 --confirm --output json`,
	}

	cmd.Flags().String("project", "",
		"Project ID or name (optional). Narrows name lookup when the same Postgres database name exists in multiple projects.")
	cmd.Flags().String("environment", "",
		"Environment ID or name (optional). Narrows name lookup when the same Postgres database name exists in multiple environments.")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input pgDeleteInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		input = normalizePGDeleteInput(input)
		confirm := command.GetConfirmFromContext(cmd.Context())

		loadData := func() (*pgDeleteResult, error) {
			pg, err := deps.PostgresService().Resolve(cmd.Context(), postgres.ResolveInput{
				IDOrName:            input.IDOrName,
				ProjectIDOrName:     input.ProjectIDOrName,
				EnvironmentIDOrName: input.EnvironmentIDOrName,
			})
			if err != nil {
				return nil, err
			}
			if confirm {
				if err := deps.PostgresService().Delete(cmd.Context(), pg.Id); err != nil {
					return nil, err
				}
			}
			return &pgDeleteResult{Postgres: pg, Deleted: confirm}, nil
		}

		_, err := command.NonInteractive(cmd,
			loadData,
			pgDeleteTextOutput,
		)
		return err
	}

	return cmd
}

func pgDeleteTextOutput(r *pgDeleteResult) string {
	if r.Deleted {
		return "Deleted this Postgres database:\n\n" + text.PostgresDetail(r.Postgres) + "\n"
	}
	return "This command would delete this Postgres database:\n\n" +
		text.PostgresDetail(r.Postgres) +
		"\n\nRe-run with --confirm to proceed\n"
}

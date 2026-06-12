package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/text"
	pgtypes "github.com/render-oss/cli/pkg/types/postgres"
)

func newPgListCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Aliases:      []string{"ls"},
		Short:        "List Postgres databases",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		Long: `List Postgres databases in the active workspace.

Use --project to narrow results to a single project, --environment to narrow
to a single environment, or both — when both are supplied, the environment is
resolved within that project.`,
		Example: `  # List all Postgres databases in the active workspace
  render ea pg list

  # List all Postgres databases in a project
  render ea pg list --project my-project

  # Filter by environment name
  render ea pg list --environment production

  # Disambiguate an environment name by project
  render ea pg list --project my-project --environment production

  # JSON output
  render ea pg list --output json`,
	}

	cmd.Flags().String("project", "",
		"Project ID or name (optional). Narrows results to environments in this project.")
	cmd.Flags().String("environment", "",
		"Environment ID or name (optional). Narrows results to this environment.")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input pgtypes.ListPostgresInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		input = pgtypes.NormalizeListInput(input)

		_, err := command.NonInteractive(cmd, func() (*postgres.PostgresListOut, error) {
			models, err := deps.PostgresService().List(cmd.Context(), input)
			if err != nil {
				return nil, err
			}
			out := postgres.NewPostgresListOut(models)
			return &out, nil
		}, func(out *postgres.PostgresListOut) string {
			return text.PostgresTable(out.Data)
		})
		return err
	}

	return cmd
}

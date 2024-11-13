package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/postgres"
	"github.com/renderinc/cli/pkg/tui"
	"github.com/renderinc/cli/pkg/tui/views"
)

// psqlCmd represents the psql command
var psqlCmd = &cobra.Command{
	Use:     "psql [postgresID]",
	Args:    cobra.MaximumNArgs(1),
	Short:   "Open a psql session to a PostgreSQL database",
	Long:    `Open a psql session to a PostgreSQL database. Optionally pass the database id as an argument.`,
	GroupID: GroupSession.ID,
}

func InteractivePSQLView(ctx context.Context, input *views.PSQLInput) tea.Cmd {
	input.Tool = views.PSQL
	return command.AddToStackFunc(
		ctx,
		psqlCmd,
		"psql",
		input,
		views.NewPSQLView(ctx, input, tui.WithCustomOptions[*postgres.Model](getPsqlTableOptions(ctx, input))),
	)
}

func getPsqlTableOptions(ctx context.Context, input *views.PSQLInput) []tui.CustomOption {
	return []tui.CustomOption{
		WithWorkspaceSelection(ctx),
		WithProjectFilter(ctx, psqlCmd, "psql", input, func(ctx context.Context, project *client.Project) tea.Cmd {
			if project != nil {
				input.Project = project
				input.EnvironmentIDs = project.EnvironmentIds
			}
			return InteractivePSQLView(ctx, input)
		}),
	}
}

func init() {
	rootCmd.AddCommand(psqlCmd)

	psqlCmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		var input views.PSQLInput
		err := command.ParseCommandInteractiveOnly(cmd, args, &input)
		if err != nil {
			return err
		}

		InteractivePSQLView(ctx, &input)
		return nil
	}
}

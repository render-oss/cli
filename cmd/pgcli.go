package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/postgres"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/renderinc/render-cli/pkg/tui/views"
)

// pgcliCmd represents the pgcli command
var pgcliCmd = &cobra.Command{
	Use:     "pgcli [postgresID]",
	Args:    cobra.MaximumNArgs(1),
	Short:   "Open a pgcli session to a PostgreSQL database",
	Long:    `Open a pgcli session to a PostgreSQL database. Optionally pass the database id as an argument.`,
	GroupID: GroupSession.ID,
}

func InteractivePGCLIView(ctx context.Context, input *views.PSQLInput) tea.Cmd {
	input.Tool = views.PGCLI
	return command.AddToStackFunc(
		ctx,
		pgcliCmd,
		"pgcli",
		input,
		views.NewPSQLView(ctx, input, tui.WithCustomOptions[*postgres.Model](getPGCLITableOptions(ctx, input))),
	)
}

func getPGCLITableOptions(ctx context.Context, input *views.PSQLInput) []tui.CustomOption {
	return []tui.CustomOption{
		WithWorkspaceSelection(ctx),
		WithProjectFilter(ctx, pgcliCmd, "pgcli", input, func(ctx context.Context, project *client.Project) tea.Cmd {
			if project != nil {
				input.Project = project
				input.EnvironmentIDs = project.EnvironmentIds
			}
			return InteractivePGCLIView(ctx, input)
		}),
	}
}

func init() {
	rootCmd.AddCommand(pgcliCmd)

	pgcliCmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		var input views.PSQLInput
		err := command.ParseCommandInteractiveOnly(cmd, args, &input)
		if err != nil {
			return err
		}

		InteractivePGCLIView(ctx, &input)
		return nil
	}
}

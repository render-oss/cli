package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/flows"
	"github.com/render-oss/cli/pkg/tui/views"
)

// psqlCmd represents the psql command
var psqlCmd = &cobra.Command{
	Use:   "psql [postgresID|postgresName]",
	Short: "Open a psql session to a PostgreSQL database",
	Long: `Open a psql session to a PostgreSQL database. Optionally pass the database id or name as an argument.
To pass arguments to psql, use the following syntax: render psql [postgresID|postgresName] -- [psql args]

For non-interactive usage, use the --command flag:
  render psql [postgresID|postgresName] -c "SELECT * FROM users;" -o text

Additional psql flags can be passed after --:
  render psql [postgresID|postgresName] -c "SELECT 1;" -o json -- --csv -q`,
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
		flows.WithCopyID(ctx, servicesCmd),
		flows.WithWorkspaceSelection(ctx),
		flows.WithProjectFilter(ctx, psqlCmd, "psql", input, func(ctx context.Context, project *client.Project) tea.Cmd {
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

	psqlCmd.Flags().StringP("command", "c", "", "SQL command to execute (enables non-interactive mode)")

	psqlCmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		var input views.PSQLInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		if cmd.ArgsLenAtDash() == 0 {
			input.PostgresIDOrName = ""
		}

		if cmd.ArgsLenAtDash() >= 0 {
			input.Args = args[cmd.ArgsLenAtDash():]
		}

		input.Tool = views.PSQL

		outputFormat := command.GetFormatFromContext(ctx)
		if outputFormat != nil && !outputFormat.Interactive() {
			if input.Command == "" {
				return fmt.Errorf("--command flag is required in non-interactive mode\nUsage: render psql <postgresID> --command \"SELECT ...\" -o json")
			}

			if input.PostgresIDOrName == "" {
				return fmt.Errorf("postgres ID or name is required in non-interactive mode")
			}

			result, err := views.ExecutePSQLNonInteractive(ctx, &input)
			if err != nil {
				return err
			}

			_, err = command.PrintData(cmd, result, text.PSQLResultText)
			return err
		}

		InteractivePSQLView(ctx, &input)
		return nil
	}
}

package cmd

import (
	"context"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/postgres"
	postgrestui "github.com/renderinc/render-cli/pkg/postgres/tui"
	"github.com/renderinc/render-cli/pkg/project"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

// psqlCmd represents the psql command
var psqlCmd = &cobra.Command{
	Use:   "psql [postgresID]",
	Args:  cobra.MaximumNArgs(1),
	Short: "Open a psql session to a Render Postgres database",
	Long:  `Open a psql session to a Render Postgres database. Optionally pass the database id as an argument.`,
}

var InteractivePSQL = command.Wrap(psqlCmd, loadDataPSQL, renderPSQL, nil)
var InteractivePSQLSelectDB = command.Wrap(psqlCmd, listDatabases, renderPSQLSelection, nil)

type PSQLInput struct {
	PostgresID string `cli:"arg:0"`
}

func loadDataPSQL(ctx context.Context, in PSQLInput) (*exec.Cmd, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	connectionInfo, err := postgres.NewRepo(c).GetPostgresConnectionInfo(ctx, in.PostgresID)
	if err != nil {
		return nil, err
	}

	return exec.Command("psql", connectionInfo.ExternalConnectionString), nil
}

func renderPSQL(ctx context.Context, loadData func(in PSQLInput) tui.TypedCmd[*exec.Cmd], in PSQLInput) (tea.Model, error) {
	return tui.NewExecModel(loadData(in)), nil
}

func listDatabases(ctx context.Context, _ PSQLInput) ([]*postgres.Model, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)
	postgresRepo := postgres.NewRepo(c)

	postgresService := postgres.NewService(postgresRepo, environmentRepo, projectRepo)

	return postgresService.ListPostgres(ctx, &client.ListPostgresParams{})
}

func renderPSQLSelection(ctx context.Context, loadData func(in PSQLInput) tui.TypedCmd[[]*postgres.Model], _ PSQLInput) (tea.Model, error) {
	columns := postgrestui.Columns()

	createRowFunc := func(p *postgres.Model) table.Row {
		return postgrestui.Row(p)
	}

	onSelect := func(rows []table.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}
		return InteractivePSQL(ctx, PSQLInput{PostgresID: rows[0].Data["ID"].(string)})
	}

	t := tui.NewTable(
		columns,
		loadData(PSQLInput{}),
		createRowFunc,
		onSelect,
	)

	return t, nil
}

func init() {
	rootCmd.AddCommand(psqlCmd)

	psqlCmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		var input PSQLInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		if input.PostgresID != "" {
			InteractivePSQL(ctx, input)
			return nil
		}

		InteractivePSQLSelectDB(ctx, input)
		return nil
	}
}

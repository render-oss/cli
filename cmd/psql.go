package cmd

import (
	"context"
	"os/exec"

	"github.com/renderinc/render-cli/pkg/resource"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/postgres"
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

var InteractivePSQL = command.Wrap(psqlCmd, loadDataPSQL, renderPSQL)
var InteractivePSQLSelectDB = command.Wrap(psqlCmd, listDatabases, renderPSQLSelection)

type PSQLInput struct {
	PostgresID string `cli:"arg:0"`
}

func loadDataPSQL(ctx context.Context, in PSQLInput) (string, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return "", err
	}

	connectionInfo, err := postgres.NewRepo(c).GetPostgresConnectionInfo(ctx, in.PostgresID)
	if err != nil {
		return "", err
	}

	return connectionInfo.ExternalConnectionString, nil
}

func renderPSQL(ctx context.Context, loadData func(in PSQLInput) (string, error), in PSQLInput) (tea.Model, error) {
	connectionString, err := loadData(in)
	if err != nil {
		return nil, err
	}

	return tui.NewExecModel(exec.Command("psql", connectionString)), nil
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

func renderPSQLSelection(ctx context.Context, loadData func(in PSQLInput) ([]*postgres.Model, error), _ PSQLInput) (tea.Model, error) {
	columns := resource.ColumnsForResources()

	loadDataFunc := func() ([]*postgres.Model, error) {
		return loadData(PSQLInput{})
	}

	createRowFunc := func(p *postgres.Model) table.Row {
		return resource.RowForResource(p)
	}

	onSelect := func(rows []table.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}
		return InteractivePSQL(ctx, PSQLInput{PostgresID: rows[0].Data["ID"].(string)})
	}

	t := tui.NewTable(
		columns,
		loadDataFunc,
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

		InteractivePSQLSelectDB(ctx, input)
		return nil
	}
}

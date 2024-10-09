/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/postgres"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

// psqlCmd represents the psql command
var psqlCmd = &cobra.Command{
	Use:   "psql [postgresID]",
	Args: cobra.ExactArgs(1),
	Short: "Open a psql session to a Render Postgres database",
	Long:  `Open a psql session to a Render Postgres database. Pass the database id as the first argument.`,
}

var InteractivePSQL = command.Wrap(psqlCmd, loadDataPSQL, renderPSQL)

type PSQLInput struct {
	PostgresID string
}

func (p PSQLInput) String() []string {
	return []string{p.PostgresID}
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

func init() {
	rootCmd.AddCommand(psqlCmd)

	psqlCmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		postgresID := args[0]

		InteractivePSQL(ctx, PSQLInput{PostgresID: postgresID})

		return nil
	}
}

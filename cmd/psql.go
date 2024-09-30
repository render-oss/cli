/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"net/http"

	"github.com/renderinc/render-cli/pkg/cfg"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/postgres"
	"github.com/spf13/cobra"
)

// psqlCmd represents the psql command
var psqlCmd = &cobra.Command{
	Use:   "psql",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		postgresID := args[0]

		c, err := client.ClientWithAuth(&http.Client{}, cfg.GetHost(), cfg.GetAPIKey())
		if err != nil {
			return err
		}
		connectionInfo, err := postgres.NewRepo(c).GetPostgresConnectionInfo(ctx, postgresID)
		if err != nil {
			return err
		}

		return command.RunProgram("psql", connectionInfo.ExternalConnectionString)
	},
}

func init() {
	rootCmd.AddCommand(psqlCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// psqlCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// psqlCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/cfg"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/logs"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		serviceID := args[0]
		command.Wrap(cmd, loadLogData, renderLogs)(cmd.Context(), LogInput{ServiceID: serviceID})
	},
}

type LogInput struct {
	ServiceID string
}

func (l LogInput) String() []string {
	return []string{l.ServiceID}
}

func loadLogData(ctx context.Context, in LogInput) (*client.Logs200Response, error) {
	c, err := client.ClientWithAuth(&http.Client{}, cfg.GetHost(), cfg.GetAPIKey())
	if err != nil {
		return nil, fmt.Errorf("error creating client: %v", err)
	}
	logRepo := logs.NewLogRepo(c)
	limit := 100
	return logRepo.ListLogs(ctx, &client.ListLogsParams{
		Resource: []string{in.ServiceID},
		OwnerId:  "tea-cdcl7qj4lvk7mmk7jbv0",
		Limit:    &limit,
	})
}

func renderLogs(ctx context.Context, loadData func() (*client.Logs200Response, error)) (tea.Model, error) {
	formattedLogs := func() ([]string, error) {
		logs, err := loadData()
		if err != nil {
			return nil, err
		}

		var formattedLogs []string
		for _, log := range logs.Logs {
			formattedLogs = append(formattedLogs, log.Message)
		}

		return formattedLogs, nil
	}
	model := tui.NewLogModel(formattedLogs)
	return model, nil
}

func init() {
	rootCmd.AddCommand(logsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// logsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// logsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/renderinc/render-cli/pkg/cfg"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/logs"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

var timeStyle = lipgloss.NewStyle().PaddingRight(2)

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
}

var InteractiveLogs = command.Wrap(logsCmd, loadLogData, renderLogs)

type LogInput struct {
	OwnerID     string   `cli:"owner"`
	ResourceIDs []string `cli:"resources"`
	StartTime   *string  `cli:"start"`
	EndTime     *string  `cli:"end"`
}

func (l LogInput) String() []string {
	return []string{}
}

func (l LogInput) ToParam() *client.ListLogsParams {
	limit := 100
	now := time.Now()
	return &client.ListLogsParams{
		Resource:  l.ResourceIDs,
		OwnerId:   l.OwnerID,
		Limit:     &limit,
		StartTime: command.ParseTime(now, l.StartTime),
		EndTime:   command.ParseTime(now, l.EndTime),
	}
}

func loadLogData(ctx context.Context, in LogInput) (*client.Logs200Response, error) {
	c, err := client.ClientWithAuth(&http.Client{}, cfg.GetHost(), cfg.GetAPIKey())
	if err != nil {
		return nil, fmt.Errorf("error creating client: %v", err)
	}
	logRepo := logs.NewLogRepo(c)
	return logRepo.ListLogs(ctx, in.ToParam())
}

func logForm(ctx context.Context, in LogInput) *tui.FilterModel {
	form, result := command.HuhForm(logsCmd, &in)
	return tui.NewFilterModel(form.WithHeight(10), func(form *huh.Form) tea.Cmd {
		var logInput LogInput
		err := command.StructFromFormValues(result, &logInput)
		if err != nil {
			panic(err)
		}

		return command.Wrap(logsCmd, loadLogData, renderLogs)(ctx, logInput)
	})
}

func formatLogs(data *client.Logs200Response) []string {
	var formattedLogs []string
	for _, log := range data.Logs {
		formattedLogs = append(formattedLogs, lipgloss.JoinHorizontal(
			lipgloss.Top,
			timeStyle.Render(log.Timestamp.Format(time.DateTime)),
			log.Message,
		))
	}

	return formattedLogs
}

func renderLogs(ctx context.Context, loadData func(LogInput) (*client.Logs200Response, error), in LogInput) (tea.Model, error) {
	formattedLogs := func() ([]string, error) {
		logs, err := loadData(in)
		if err != nil {
			return nil, err
		}

		return formatLogs(logs), nil
	}
	model := tui.NewLogModel(logForm(ctx, in), formattedLogs)
	return model, nil
}

func init() {
	logsCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input LogInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}
		InteractiveLogs(cmd.Context(), input)
		return nil
	}
	rootCmd.AddCommand(logsCmd)

	logsCmd.Flags().StringSliceP("resources", "r", []string{}, "A list of comma separated resource IDs to query")
	err := logsCmd.MarkFlagRequired("resources")
	if err != nil {
		panic(err)
	}

	logsCmd.Flags().String("owner", "", "The owner ID of the resources to query")
	err = logsCmd.MarkFlagRequired("owner")
	if err != nil {
		panic(err)
	}

	logsCmd.Flags().String("start", "", "The start time of the logs to query")
	logsCmd.Flags().String("end", "", "The end time of the logs to query")
}

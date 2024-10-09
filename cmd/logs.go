/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/renderinc/render-cli/pkg/client"
	lclient "github.com/renderinc/render-cli/pkg/client/logs"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/config"
	"github.com/renderinc/render-cli/pkg/logs"
	"github.com/renderinc/render-cli/pkg/pointers"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const defaultLogLimit = 100

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View logs for services, cron jobs, and databases",
	Long: `View logs for services, cron jobs, and databases.

Use flags to filter logs by resource, instance, time, text, level, type, host, status code, method, or path.
Unlike in the dashboard you can view logs for multiple resources at once. Set --tail=true to stream new logs (currently only in interactive mode).

In interactive mode you can update the filters and view logs in real time.`,
}

var InteractiveLogs = command.Wrap(logsCmd, loadLogData, renderLogs)

type LogInput struct {
	ResourceIDs []string `cli:"resources"`
	Instance    []string `cli:"instance"`
	StartTime   *string  `cli:"start"`
	EndTime     *string  `cli:"end"`
	Text        []string `cli:"text"`
	Level       []string `cli:"level"`
	Type        []string `cli:"type"`

	Host       []string `cli:"host"`
	StatusCode []string `cli:"status-code"`
	Method     []string `cli:"method"`
	Path       []string `cli:"path"`

	Limit     int    `cli:"limit"`
	Direction string `cli:"direction"`
	Tail      bool   `cli:"tail"`
}

type LogResult struct {
	Logs       *client.Logs200Response
	LogChannel <-chan *lclient.Log
}

func (l LogInput) String() []string {
	return []string{}
}

func (l LogInput) ToParam() (*client.ListLogsParams, error) {
	now := time.Now()
	ownerID, err := config.WorkspaceID()
	if err != nil {
		return nil, fmt.Errorf("error getting workspace ID: %v", err)
	}

	if l.Limit == 0 {
		l.Limit = 100
	}

	return &client.ListLogsParams{
		Resource:   l.ResourceIDs,
		OwnerId:    ownerID,
		Instance:   pointers.FromArray(l.Instance),
		Limit:      pointers.From(l.Limit),
		StartTime:  command.ParseTime(now, l.StartTime),
		EndTime:    command.ParseTime(now, l.EndTime),
		Text:       pointers.FromArray(l.Text),
		Level:      pointers.FromArray(l.Level),
		Type:       pointers.FromArray(l.Type),
		Host:       pointers.FromArray(l.Host),
		StatusCode: pointers.FromArray(l.StatusCode),
		Method:     pointers.FromArray(l.Method),
		Path:       pointers.FromArray(l.Path),
		Direction:  pointers.From(mapDirection(l.Direction)),
	}, nil
}

func mapDirection(direction string) lclient.LogDirection {
	switch direction {
	case "forward":
		return lclient.Forward
	case "backward":
		return lclient.Backward
	default:
		return lclient.Backward
	}
}

func loadLogData(ctx context.Context, in LogInput) (*LogResult, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("error creating client: %v", err)
	}
	logRepo := logs.NewLogRepo(c)
	params, err := in.ToParam()
	if err != nil {
		return nil, fmt.Errorf("error converting input to params: %v", err)
	}

	if in.Tail {
		logChan, err := logRepo.TailLogs(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("error tailing logs: %v", err)
		}
		return &LogResult{Logs: &client.Logs200Response{}, LogChannel: logChan}, nil
	}

	logs, err := logRepo.ListLogs(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("error listing logs: %v", err)
	}
	return &LogResult{Logs: logs, LogChannel: nil}, nil
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

func renderLogs(ctx context.Context, loadData func(LogInput) (*LogResult, error), in LogInput) (tea.Model, error) {
	loadLogs := func() (*client.Logs200Response, <-chan *lclient.Log, error) {
		result, err := loadData(in)
		if err != nil {
			return nil, nil, err
		}

		return result.Logs, result.LogChannel, nil
	}
	model := tui.NewLogModel(logForm(ctx, in), loadLogs)
	return model, nil
}

func writeLog(format command.Output, out io.Writer, log *lclient.Log) error {
	var str []byte
	var err error
	if format == command.JSON {
		str, err = json.MarshalIndent(log, "", "  ")
	} else if format == command.YAML {
		str, err = yaml.Marshal(log)
	}

	if err != nil {
		return err
	}

	_, err = out.Write(str)
	return err
}

func nonInteractiveLogs(format *command.Output, cmd *cobra.Command, input LogInput) error {
	result, err := loadLogData(cmd.Context(), input)
	if err != nil {
		return err
	}

	if result.Logs != nil {
		for _, log := range result.Logs.Logs {
			if err := writeLog(*format, cmd.OutOrStdout(), &log); err != nil {
				return err
			}
		}
	}

	if result.LogChannel != nil {
		for {
			log, ok := <-result.LogChannel
			if !ok {
				break
			}
			if err := writeLog(*format, cmd.OutOrStdout(), log); err != nil {
				return err
			}
		}
	}

	return nil
}

func init() {
	logsCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input LogInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		// Normally we'd let the wrapper handle non-interactive mode.
		// However, logs are a special case where we want to stream new logs
		// from the server. Since we don't have other commands that stream
		// we're going to special case this one.
		format := command.GetFormatFromContext(cmd.Context())
		if format != nil && (*format == command.JSON || *format == command.YAML) {
			return nonInteractiveLogs(format, cmd, input)
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

	logsCmd.Flags().String("start", "", "The start time of the logs to query")
	logsCmd.Flags().String("end", "", "The end time of the logs to query")
	logsCmd.Flags().StringSlice("text", []string{}, "A list of comma separated strings to search for in the logs. Only logs that contain all of the strings will be returned. Wildcards * and regular expressions are supported.")
	logsCmd.Flags().StringSlice("level", []string{}, "A list of comma separated log levels to query")
	logsCmd.Flags().StringSlice("type", []string{}, "A list of comma separated log types to query")
	logsCmd.Flags().StringSlice("instance", []string{}, "A list of comma separated instance IDs to query")
	logsCmd.Flags().StringSlice("host", []string{}, "A list of comma separated hosts to query")
	logsCmd.Flags().StringSlice("status-code", []string{}, "A list of comma separated status codes to query")
	logsCmd.Flags().StringSlice("method", []string{}, "A list of comma separated HTTP methods to query")
	logsCmd.Flags().StringSlice("path", []string{}, "A list of comma separated paths to query")
	logsCmd.Flags().Int("limit", defaultLogLimit, "The maximum number of logs to return")
	logsCmd.Flags().String("direction", "backward", "The direction to query the logs. Can be 'forward' or 'backward'")

	logsCmd.Flags().Bool("tail", false, "Stream new logs")
}

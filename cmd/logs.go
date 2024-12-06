package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/renderinc/cli/pkg/client"
	lclient "github.com/renderinc/cli/pkg/client/logs"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/pointers"
	"github.com/renderinc/cli/pkg/resource"
	"github.com/renderinc/cli/pkg/tui"
	"github.com/renderinc/cli/pkg/tui/views"
)

var LogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View logs for services and datastores",
	Long: `View logs for services and datastores.

Use flags to filter logs by resource, instance, time, text, level, type, host, status code, method, or path.
Unlike in the dashboard, you can view logs for multiple resources at once. Set --tail=true to stream new logs (currently only in interactive mode).

In interactive mode you can update the filters and view logs in real time.`,
	GroupID: GroupCore.ID,
}

func filterLogs(ctx context.Context, in views.LogInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, LogsCmd, breadcrumb, &in, views.NewLogsView(ctx, LogsCmd, filterLogs, in, views.LoadLogData))
}

func writeLog(format command.Output, out io.Writer, log *lclient.Log) error {
	var str []byte
	var err error
	if format == command.JSON {
		str, err = json.MarshalIndent(log, "", "  ")
	} else if format == command.YAML {
		str, err = yaml.Marshal(log)
	} else if format == command.TEXT {
		str = []byte(fmt.Sprintf("%s  %s\n", log.Timestamp.Format(time.DateTime), log.Message))
	}

	if err != nil {
		return err
	}

	_, err = out.Write(str)
	return err
}

func nonInteractiveLogs(format *command.Output, cmd *cobra.Command, input views.LogInput) error {
	result, err := views.LoadLogData(cmd.Context(), input)
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

func TailResourceLogs(ctx context.Context, resourceID string) tea.Cmd {
	return InteractiveLogs(
		ctx,
		views.LogInput{
			StartTime:   &command.TimeOrRelative{T: pointers.From(time.Now())},
			ResourceIDs: []string{resourceID},
			Tail:        true,
		}, "Logs")
}

func InteractiveLogs(ctx context.Context, input views.LogInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(
		ctx,
		LogsCmd,
		breadcrumb,
		&input,
		views.NewLogsView(ctx, LogsCmd, filterLogs, input, views.LoadLogData, tui.WithCustomOptions[resource.Resource](getLogsOptions(ctx, breadcrumb))),
	)
}

func getLogsOptions(ctx context.Context, breadcrumb string) []tui.CustomOption {
	return []tui.CustomOption{
		WithWorkspaceSelection(ctx),
		WithProjectFilter(ctx, servicesCmd, "Project Filter", &views.LogInput{}, func(ctx context.Context, project *client.Project) tea.Cmd {
			logInput := views.LogInput{}
			if project != nil {
				logInput.ListResourceInput.Project = project
				logInput.ListResourceInput.EnvironmentIDs = project.EnvironmentIds
			}
			return InteractiveLogs(ctx, logInput, breadcrumb)
		}),
	}
}

func init() {
	directionFlag := command.NewEnumInput([]string{"backward", "forward"}, false)
	levelFlag := command.NewEnumInput([]string{
		"debug", "info", "notice", "warning", "error", "critical", "alert", "emergency",
	}, true)
	logTypeFlag := command.NewEnumInput([]string{"app", "request", "build"}, true)
	methodTypeFlag := command.NewEnumInput([]string{
		"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD", "CONNECT", "TRACE",
	}, true)

	startTimeFlag := command.NewTimeInput()
	endTimeFlag := command.NewTimeInput()

	LogsCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input views.LogInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		format := command.GetFormatFromContext(cmd.Context())
		if format != nil && (*format != command.Interactive) {
			return nonInteractiveLogs(format, cmd, input)
		}

		InteractiveLogs(cmd.Context(), input, "Logs")
		return nil
	}

	LogsCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// Resources flag is required in non-interactive mode
		format := command.GetFormatFromContext(cmd.Context())
		if format != nil && *format != command.Interactive {
			return LogsCmd.MarkFlagRequired("resources")
		}
		return nil
	}

	rootCmd.AddCommand(LogsCmd)

	LogsCmd.Flags().StringSliceP("resources", "r", []string{}, "A list of comma separated resource IDs to query. Required in non-interactive mode.")
	LogsCmd.Flags().Var(startTimeFlag, "start", "The start time of the logs to query")
	LogsCmd.Flags().Var(endTimeFlag, "end", "The end time of the logs to query")
	LogsCmd.Flags().StringSlice("text", []string{}, "A list of comma separated strings to search for in the logs")
	LogsCmd.Flags().Var(levelFlag, "level", "A list of comma separated log levels to query")
	LogsCmd.Flags().Var(logTypeFlag, "type", "A list of comma separated log types to query")
	LogsCmd.Flags().StringSlice("instance", []string{}, "A list of comma separated instance IDs to query")
	LogsCmd.Flags().StringSlice("host", []string{}, "A list of comma separated hosts to query")
	LogsCmd.Flags().StringSlice("status-code", []string{}, "A list of comma separated status codes to query")
	LogsCmd.Flags().Var(methodTypeFlag, "method", "A list of comma separated HTTP methods to query")
	LogsCmd.Flags().StringSlice("path", []string{}, "A list of comma separated paths to query")
	LogsCmd.Flags().Int("limit", 100, "The maximum number of logs to return")
	LogsCmd.Flags().Var(directionFlag, "direction", "The direction to query the logs. Can be 'forward' or 'backward'")
	LogsCmd.Flags().Bool("tail", false, "Stream new logs")
}

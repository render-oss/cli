package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	lclient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/logs"
	"github.com/render-oss/cli/pkg/tui/flows"
	"github.com/render-oss/cli/pkg/tui/views"
)

func NewLogsCmd(deps flows.LogFlowDeps) *cobra.Command {
	logCmd := &cobra.Command{
		Use:   "logs",
		Short: "View logs for services and datastores",
		Long: `View logs for services and datastores.

Use flags to filter logs by resource, instance, time, text, level, type, host, status code, method, or path.
Unlike in the dashboard, you can view logs for multiple resources at once. Set --tail=true to stream new logs (currently only in interactive mode).

In interactive mode you can update the filters and view logs in real time.`,
		GroupID: GroupCore.ID,
		RunE: func(cmd *cobra.Command, args []string) error {
			var input views.LogInput
			err := command.ParseCommand(cmd, args, &input)
			if err != nil {
				return err
			}

			format := command.GetFormatFromContext(cmd.Context())
			if format != nil && (*format != command.Interactive) {
				return nonInteractiveLogs(deps.LogLoader(), format, cmd, input)
			}

			flows.NewLogFlow(deps).LogsFlow(cmd.Context(), input)
			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Resources flag is required in non-interactive mode
			format := command.GetFormatFromContext(cmd.Context())
			if format != nil && *format != command.Interactive {
				return deps.LogsCmd().MarkFlagRequired("resources")
			}
			return nil
		},
	}

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

	logCmd.Flags().StringSliceP("resources", "r", []string{}, "A list of comma separated resource IDs to query. Required in non-interactive mode.")
	logCmd.Flags().Var(startTimeFlag, "start", "The start time of the logs to query")
	logCmd.Flags().Var(endTimeFlag, "end", "The end time of the logs to query")
	logCmd.Flags().StringSlice("text", []string{}, "A list of comma separated strings to search for in the logs")
	logCmd.Flags().Var(levelFlag, "level", "A list of comma separated log levels to query")
	logCmd.Flags().Var(logTypeFlag, "type", "A list of comma separated log types to query")
	logCmd.Flags().StringSlice("instance", []string{}, "A list of comma separated instance IDs to query")
	logCmd.Flags().StringSlice("host", []string{}, "A list of comma separated hosts to query")
	logCmd.Flags().StringSlice("status-code", []string{}, "A list of comma separated status codes to query")
	logCmd.Flags().Var(methodTypeFlag, "method", "A list of comma separated HTTP methods to query")
	logCmd.Flags().StringSlice("path", []string{}, "A list of comma separated paths to query")
	logCmd.Flags().Int("limit", logs.DefaultLogLimit, "The maximum number of logs to return")
	logCmd.Flags().Var(directionFlag, "direction", "The direction to query the logs. Can be 'forward' or 'backward'")
	logCmd.Flags().Bool("tail", false, "Stream new logs")
	logCmd.Flags().StringSlice("task-id", []string{}, "A list of comma separated task IDs to query")
	logCmd.Flags().StringSlice("task-run-id", []string{}, "A list of comma separated task run IDs to query")

	return logCmd
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

func nonInteractiveLogs(logLoader *views.LogLoader, format *command.Output, cmd *cobra.Command, input views.LogInput) error {
	result, err := logLoader.LoadLogData(cmd.Context(), input)
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

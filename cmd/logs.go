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

Use flags to filter logs by resource, instance, time, text, level, type, host, status code, method, or path. Unlike in the Render Dashboard, you can view logs for multiple resources at once.

In interactive mode you can update the filters and view logs in real time, or set --tail=true to stream new logs.`,
		GroupID: GroupCore.ID,
		Example: `  # Tail logs for a service
  render logs --resources srv-abc123 --tail

  # Query logs in a time range
  render logs --resources srv-abc123 --start 2026-03-01T00:00:00Z --end 2026-03-01T01:00:00Z

  # Output logs as JSON in non-interactive mode
  render logs --resources srv-abc123 --output json`,
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

	logCmd.Flags().StringSliceP("resources", "r", []string{}, "Filter logs by comma-separated resource IDs (Required in non-interactive mode)")
	logCmd.Flags().Var(startTimeFlag, "start", "Filter logs at or after the specified start time")
	logCmd.Flags().Var(endTimeFlag, "end", "Filter logs at or before the specified end time")
	logCmd.Flags().StringSlice("text", []string{}, "Filter logs by comma-separated text values")
	logCmd.Flags().Var(levelFlag, "level", "Filter logs by comma-separated log levels")
	logCmd.Flags().Var(logTypeFlag, "type", "Filter logs by comma-separated log types")
	logCmd.Flags().StringSlice("instance", []string{}, "Filter logs by comma-separated instance IDs")
	logCmd.Flags().StringSlice("host", []string{}, "Filter logs by comma-separated host values")
	logCmd.Flags().StringSlice("status-code", []string{}, "Filter logs by comma-separated status codes")
	logCmd.Flags().Var(methodTypeFlag, "method", "Filter logs by comma-separated HTTP methods")
	logCmd.Flags().StringSlice("path", []string{}, "Filter logs by comma-separated request paths")
	logCmd.Flags().Int("limit", logs.DefaultLogLimit, "Limit the number of logs returned")
	logCmd.Flags().Var(directionFlag, "direction", "Set log query direction (backward or forward)")
	logCmd.Flags().Bool("tail", false, "Stream new logs")
	logCmd.Flags().StringSlice("task-id", []string{}, "Filter logs by comma-separated task IDs")
	logCmd.Flags().StringSlice("task-run-id", []string{}, "Filter logs by comma-separated task run IDs")
	setAnnotationBestEffort(logCmd.Flags(), "resources", command.FlagPlaceholderAnnotation, []string{"RESOURCE_IDS"})
	setAnnotationBestEffort(logCmd.Flags(), "start", command.FlagPlaceholderAnnotation, []string{"TIME"})
	setAnnotationBestEffort(logCmd.Flags(), "end", command.FlagPlaceholderAnnotation, []string{"TIME"})
	setAnnotationBestEffort(logCmd.Flags(), "text", command.FlagPlaceholderAnnotation, []string{"QUERY_TEXT"})
	setAnnotationBestEffort(logCmd.Flags(), "level", command.FlagPlaceholderAnnotation, []string{"LOG_LEVEL"})
	setAnnotationBestEffort(logCmd.Flags(), "type", command.FlagPlaceholderAnnotation, []string{"LOG_TYPE"})
	setAnnotationBestEffort(logCmd.Flags(), "instance", command.FlagPlaceholderAnnotation, []string{"INSTANCE_IDS"})
	setAnnotationBestEffort(logCmd.Flags(), "host", command.FlagPlaceholderAnnotation, []string{"HOSTS"})
	setAnnotationBestEffort(logCmd.Flags(), "status-code", command.FlagPlaceholderAnnotation, []string{"STATUS_CODES"})
	setAnnotationBestEffort(logCmd.Flags(), "method", command.FlagPlaceholderAnnotation, []string{"HTTP_METHOD"})
	setAnnotationBestEffort(logCmd.Flags(), "path", command.FlagPlaceholderAnnotation, []string{"PATHS"})
	setAnnotationBestEffort(logCmd.Flags(), "limit", command.FlagPlaceholderAnnotation, []string{"COUNT"})
	setAnnotationBestEffort(logCmd.Flags(), "direction", command.FlagPlaceholderAnnotation, []string{"LOG_DIRECTION"})
	setAnnotationBestEffort(logCmd.Flags(), "task-id", command.FlagPlaceholderAnnotation, []string{"TASK_IDS"})
	setAnnotationBestEffort(logCmd.Flags(), "task-run-id", command.FlagPlaceholderAnnotation, []string{"TASK_RUN_IDS"})

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

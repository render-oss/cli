package cmd

import (
	"context"
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
	logCmd.MarkFlagsMutuallyExclusive("tail", "end")
	setFlagPlaceholder(logCmd.Flags(), "resources", "RESOURCE_IDS")
	setFlagPlaceholder(logCmd.Flags(), "start", "TIME")
	setFlagPlaceholder(logCmd.Flags(), "end", "TIME")
	setFlagPlaceholder(logCmd.Flags(), "text", "QUERY_TEXT")
	setFlagPlaceholder(logCmd.Flags(), "level", "LOG_LEVEL")
	setFlagPlaceholder(logCmd.Flags(), "type", "LOG_TYPE")
	setFlagPlaceholder(logCmd.Flags(), "instance", "INSTANCE_IDS")
	setFlagPlaceholder(logCmd.Flags(), "host", "HOSTS")
	setFlagPlaceholder(logCmd.Flags(), "status-code", "STATUS_CODES")
	setFlagPlaceholder(logCmd.Flags(), "method", "HTTP_METHOD")
	setFlagPlaceholder(logCmd.Flags(), "path", "PATHS")
	setFlagPlaceholder(logCmd.Flags(), "limit", "COUNT")
	setFlagPlaceholder(logCmd.Flags(), "direction", "LOG_DIRECTION")
	setFlagPlaceholder(logCmd.Flags(), "task-id", "TASK_IDS")
	setFlagPlaceholder(logCmd.Flags(), "task-run-id", "TASK_RUN_IDS")

	return logCmd
}

// logWriter writes log entries in a non-interactive output format. A single
// logWriter must be used for every entry of one command invocation so that YAML
// output forms a valid multi-document stream (entries separated by "---")
// instead of concatenated mappings with duplicate keys.
type logWriter struct {
	format      command.Output
	out         io.Writer
	yamlEncoder *yaml.Encoder
}

func newLogWriter(format command.Output, out io.Writer) *logWriter {
	return &logWriter{format: format, out: out}
}

func (w *logWriter) write(log *lclient.Log) error {
	var str []byte
	var err error
	if w.format == command.JSON {
		str, err = json.MarshalIndent(log, "", "  ")
	} else if w.format == command.YAML {
		// The encoder is created lazily so that zero entries produce an empty
		// stream. Each Encode call flushes its document, preserving streaming.
		if w.yamlEncoder == nil {
			w.yamlEncoder = yaml.NewEncoder(w.out)
		}
		return w.yamlEncoder.Encode(log)
	} else if w.format == command.TEXT {
		str = []byte(fmt.Sprintf("%s  %s\n", log.Timestamp.Format(time.DateTime), log.Message))
	}

	if err != nil {
		return err
	}

	_, err = w.out.Write(str)
	return err
}

func (w *logWriter) close() error {
	if w.yamlEncoder == nil {
		return nil
	}
	return w.yamlEncoder.Close()
}

func nonInteractiveLogs(logLoader *views.LogLoader, format *command.Output, cmd *cobra.Command, input views.LogInput) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	w := newLogWriter(*format, cmd.OutOrStdout())

	if !input.Tail {
		result, err := logLoader.ListLogs(ctx, input)
		if err != nil {
			return err
		}
		if result == nil {
			return nil
		}
		for _, log := range result.Logs {
			if err := w.write(&log); err != nil {
				return err
			}
		}
		return w.close()
	}

	events, err := logLoader.TailLogs(ctx, input)
	if err != nil {
		return err
	}
	for event := range events {
		if event.Err != nil {
			return event.Err
		}
		if event.Log != nil {
			if err := w.write(event.Log); err != nil {
				return err
			}
		} else if event.Status != "" {
			if _, err := fmt.Fprintln(cmd.ErrOrStderr(), event.Status); err != nil {
				return err
			}
		}
	}
	if err := w.close(); err != nil {
		return err
	}
	return ctx.Err()
}

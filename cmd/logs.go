package cmd

import (
	"context"
	"encoding/json"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/renderinc/render-cli/pkg/client"
	lclient "github.com/renderinc/render-cli/pkg/client/logs"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/resource"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/renderinc/render-cli/pkg/tui/views"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View logs for services, cron jobs, and databases",
	Long: `View logs for services, cron jobs, and databases.

Use flags to filter logs by resource, instance, time, text, level, type, host, status code, method, or path.
Unlike in the dashboard you can view logs for multiple resources at once. Set --tail=true to stream new logs (currently only in interactive mode).

In interactive mode you can update the filters and view logs in real time.`,
}

func filterLogs(ctx context.Context, in views.LogInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, logsCmd, breadcrumb, &in, views.NewLogsView(ctx, logsCmd, filterLogs, in))
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

func InteractiveLogs(ctx context.Context, input views.LogInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(
		ctx,
		logsCmd,
		breadcrumb,
		&input,
		views.NewLogsView(ctx, logsCmd, filterLogs, input, tui.WithCustomOptions[resource.Resource](getLogsOptions(ctx, breadcrumb))),
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
	logsCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input views.LogInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		format := command.GetFormatFromContext(cmd.Context())
		if format != nil && (*format == command.JSON || *format == command.YAML) {
			return nonInteractiveLogs(format, cmd, input)
		}

		InteractiveLogs(cmd.Context(), input, "Logs")
		return nil
	}

	logsCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// Resources flag is required in non-interactive mode
		format := command.GetFormatFromContext(cmd.Context())
		if format != nil && *format != command.Interactive {
			return logsCmd.MarkFlagRequired("resources")
		}
		return nil
	}

	rootCmd.AddCommand(logsCmd)

	logsCmd.Flags().StringSliceP("resources", "r", []string{}, "A list of comma separated resource IDs to query. Required in non-interactive mode.")
	logsCmd.Flags().String("start", "", "The start time of the logs to query")
	logsCmd.Flags().String("end", "", "The end time of the logs to query")
	logsCmd.Flags().StringSlice("text", []string{}, "A list of comma separated strings to search for in the logs")
	logsCmd.Flags().StringSlice("level", []string{}, "A list of comma separated log levels to query")
	logsCmd.Flags().StringSlice("type", []string{}, "A list of comma separated log types to query")
	logsCmd.Flags().StringSlice("instance", []string{}, "A list of comma separated instance IDs to query")
	logsCmd.Flags().StringSlice("host", []string{}, "A list of comma separated hosts to query")
	logsCmd.Flags().StringSlice("status-code", []string{}, "A list of comma separated status codes to query")
	logsCmd.Flags().StringSlice("method", []string{}, "A list of comma separated HTTP methods to query")
	logsCmd.Flags().StringSlice("path", []string{}, "A list of comma separated paths to query")
	logsCmd.Flags().Int("limit", 100, "The maximum number of logs to return")
	logsCmd.Flags().String("direction", "backward", "The direction to query the logs. Can be 'forward' or 'backward'")
	logsCmd.Flags().Bool("tail", false, "Stream new logs")
}

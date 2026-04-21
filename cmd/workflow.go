package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
	"github.com/render-oss/cli/pkg/command"
	renderstyle "github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/workflows/apiserver"
	logstore "github.com/render-oss/cli/pkg/workflows/logs"
	"github.com/render-oss/cli/pkg/workflows/orchestrator"
	"github.com/render-oss/cli/pkg/workflows/store"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
	"github.com/spf13/cobra"
)

const defaultTaskAPIPort = 8120

var WorkflowsCmd = &cobra.Command{
	Use:   "workflows",
	Short: "Manage workflows",
	Long: `Manage workflow services for the active workspace.

List workflows, browse versions and tasks, start task runs, and trigger releases.`,
	GroupID: GroupCore.ID,
}

var workflowDevCmd = &cobra.Command{
	Use:          "dev -- <command to start a workflow service>",
	Short:        "Start a workflows service in development mode",
	SilenceUsage: true,
	Long: `Start a workflow service in development mode for local testing.

This command runs your workflow service locally on port 8120, allowing you to list and run tasks without deploying to Render. Task runs and their logs are stored in memory, so you can query them after tasks complete.

The command will spawn a new subprocess with your specified command whenever it needs to run a task or list the defined tasks.

To interact with the local task server:
  • Use the --local flag with other task commands (e.g., 'render workflows tasks list --local')
  • Or set RENDER_USE_LOCAL_DEV=true when using the workflow client SDK

To use a different port:
  • Specify --port when starting the dev server
  • Then use --port with other task commands, or set RENDER_LOCAL_DEV_URL in the SDK

Examples:
  render workflows dev -- "python main.py"
  render workflows dev --port 9000 -- "npm start"
  render workflows tasks list --local
  render workflows tasks start my-task --local --input='["arg1"]'
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		var commandArgs []string
		if cmd.ArgsLenAtDash() >= 0 {
			commandArgs = args[cmd.ArgsLenAtDash():]
		}

		if len(commandArgs) == 0 {
			return errors.New("command is required")
		}

		debugMode, err := cmd.Flags().GetBool("debug")
		if err != nil {
			return fmt.Errorf("failed to get debug flag: %w", err)
		}

		port, err := cmd.Flags().GetInt("port")
		if err != nil {
			return fmt.Errorf("failed to get port flag: %w", err)
		}

		socketTracker, err := orchestrator.NewSocketTracker(ctx)
		if err != nil {
			return err
		}

		taskServerFactory := taskserver.NewTaskServerFactory()

		logs := logstore.NewLogStore()
		store := store.NewTaskStore()
		var (
			pending []string
			ready   bool
		)

		statusReporter := orchestrator.NewPrintStatusReporter(
			func(format string, args ...any) {
				message := fmt.Sprintf(format, args...)
				if !ready {
					pending = append(pending, message)
					return
				}
				command.Println(cmd, "%s", message)
			},
			orchestrator.WithStatusReporterTimestamps(debugMode),
			orchestrator.WithStatusReporterTaskEnqueued(debugMode),
			orchestrator.WithStatusReporterIncludeInputs(true),
		)
		coordinator := orchestrator.NewCoordinator(
			ctx,
			store,
			orchestrator.NewExec(logs, debugMode, commandArgs[0], commandArgs[1:]...),
			socketTracker,
			taskServerFactory,
			statusReporter,
		)

		upgrader := &websocket.Upgrader{
			Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
				http.Error(w, "failed to upgrade http connection to websocket", http.StatusUpgradeRequired)
			},
		}

		api := apiserver.NewHandler(coordinator, store, logs, upgrader)
		apiSrv, err := apiserver.Start(api, port)
		if err != nil {
			if errors.Is(err, syscall.EADDRINUSE) {
				return fmt.Errorf("port %d is already in use. Stop the other process or use --port to pick a different one", port)
			}
			return fmt.Errorf("failed to start server on port %d: %w", port, err)
		}

		ok := lipgloss.NewStyle().Foreground(renderstyle.ColorOK)
		info := lipgloss.NewStyle().Foreground(renderstyle.ColorInfo)
		dim := lipgloss.NewStyle().Foreground(renderstyle.ColorDeprioritized)

		command.Println(cmd, "%s %s",
			ok.Render("Workflow server listening on port"),
			renderstyle.Bold(fmt.Sprintf("%d", port)),
		)

		logs.Start(ctx)

		registeredTasks, err := coordinator.PopulateTasks(ctx)
		if err != nil {
			return fmt.Errorf("failed to load tasks: %w", err)
		}

		command.Println(cmd, "%s", formatTaskSummary(info, registeredTasks, describeWorkflowSource(commandArgs)))
		command.Println(cmd, "")

		portFlag := ""
		if port != defaultTaskAPIPort {
			portFlag = fmt.Sprintf(" --port %d", port)
		}
		command.Println(cmd, "%s", dim.Render("To browse and run tasks, open another terminal and run:"))
		command.Println(cmd, "  %s", renderstyle.Bold(fmt.Sprintf("render workflows tasks list --local%s", portFlag)))
		command.Println(cmd, "")
		command.Println(cmd, "%s", dim.Render("To trigger a specific task directly:"))
		command.Println(cmd, "  %s", renderstyle.Bold(fmt.Sprintf("render workflows tasks start <task-name> --local%s --input='[\"arg1\"]'", portFlag)))
		command.Println(cmd, "")

		ready = true
		for _, line := range pending {
			command.Println(cmd, "%s", line)
		}
		pending = nil

		<-ctx.Done()

		apiSrv.Shutdown(ctx)

		return nil
	},
	Args: cobra.MinimumNArgs(1),
}

func init() {
	workflowDevCmd.Flags().Int("port", defaultTaskAPIPort, "Port of the local task server (8120 when not specified)")
	workflowDevCmd.Flags().Bool("debug", false, "Print detailed workflow task execution events")

	rootCmd.AddCommand(WorkflowsCmd)
	WorkflowsCmd.AddCommand(workflowDevCmd)
}

func describeWorkflowSource(commandArgs []string) string {
	if len(commandArgs) == 0 {
		return ""
	}

	last := commandArgs[len(commandArgs)-1]
	base := filepath.Base(last)
	if strings.Contains(base, ".") {
		return base
	}

	return strings.Join(commandArgs, " ")
}

func formatTaskSummary(info lipgloss.Style, tasks []*store.Task, source string) string {
	if len(tasks) == 0 {
		return fmt.Sprintf("0 tasks found in %s (waiting for registration)", renderstyle.Bold(source))
	}

	names := make([]string, 0, len(tasks))
	for _, task := range tasks {
		names = append(names, task.Name)
	}
	sort.Strings(names)

	plural := ""
	if len(names) > 1 {
		plural = "s"
	}

	header := fmt.Sprintf("%d task%s found in %s", len(names), plural, renderstyle.Bold(source))

	var b strings.Builder
	b.WriteString(header)
	for _, name := range names {
		b.WriteString("\n  • ")
		b.WriteString(info.Render(name))
	}
	return b.String()
}

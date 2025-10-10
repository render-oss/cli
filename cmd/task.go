package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/render-oss/cli/pkg/client"
	workflows "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/logs"
	"github.com/render-oss/cli/pkg/tasks"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/flows"
	"github.com/render-oss/cli/pkg/tui/views"
	workflowviews "github.com/render-oss/cli/pkg/tui/views/workflows"
	"github.com/render-oss/cli/pkg/workflows/apiserver"
	logstore "github.com/render-oss/cli/pkg/workflows/logs"
	"github.com/render-oss/cli/pkg/workflows/orchestrator"
	"github.com/render-oss/cli/pkg/workflows/store"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
	"github.com/spf13/cobra"
)

const defaultTaskAPIPort = 8120

var taskCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Manage tasks",
}

var taskDevCmd = &cobra.Command{
	Use:   "dev -- <command to start a workflow service>",
	Short: "Start a workflow service in development mode",
	Long: `Start a workflow service in development mode for local testing.

This command runs your workflow service locally on port 8120, allowing you to list and run
tasks without deploying to Render. Task runs and their logs are stored in memory, so you
can query them after tasks complete.

The command will spawn a new subprocess with your specified command whenever it needs to
run a task or list the defined tasks.

To interact with the local task server:
  • Use the --local flag with other task commands (e.g., 'render tasks list --local')
  • Or set RENDER_USE_LOCAL_DEV=true when using the workflow client SDK

To use a different port:
  • Specify --port when starting the dev server
  • Then use --port with other task commands, or set RENDER_LOCAL_DEV_URL in the SDK

Examples:
  render ea tasks dev -- "go run main.go"
  render ea tasks dev --port 9000 -- "npm start"
  render ea tasks list --local
  render ea taskruns start my-task --local --input='["arg1"]'
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

		socketTracker, err := orchestrator.NewSocketTracker(ctx)
		if err != nil {
			return err
		}

		taskServerFactory := taskserver.NewTaskServerFactory()

		logs := logstore.NewLogStore()
		store := store.NewTaskStore()
		coordinator := orchestrator.NewCoordinator(ctx, store, orchestrator.NewExec(logs, commandArgs[0], commandArgs[1:]...), socketTracker, taskServerFactory)

		upgrader := &websocket.Upgrader{
			Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
				http.Error(w, "failed to upgrade http connection to websocket", http.StatusUpgradeRequired)
			},
		}

		port, err := cmd.Flags().GetInt("port")
		if err != nil {
			return fmt.Errorf("failed to get port flag: %w", err)
		}

		api := apiserver.NewHandler(coordinator, store, logs, upgrader)
		apiSrv := apiserver.Start(api, port)
		logs.Start(ctx)

		<-ctx.Done()

		apiSrv.Shutdown(ctx)

		return nil
	},
	Args: cobra.MinimumNArgs(1),
}

func NewTaskRunStartCmd(deps flows.WorkflowDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [taskID] --input=<json>",
		Short: "Start a task run with the provided input",
		Long: `Start a task with the provided input.

You can specify the task by:
  • Task ID (e.g., tsk-1234)
  • Workflow slug and task name (e.g., my-workflow/my-task)

Input Format:
The input should be a JSON array where each element is an argument to the task.
For example, if your task takes two arguments, provide: ["arg1", "arg2"]

You can provide input via:
  • --input with inline JSON
  • --input-file with a path to a JSON file

In interactive mode, you will be prompted to select the task and provide the input.

Examples:
  render ea taskruns start tsk-1234 --input='["arg1", "arg2"]'
  render ea taskruns start my-workflow/my-task --input='[42, "hello"]'
  render ea taskruns start my-task --input-file=input.json
  render ea taskruns start my-task --local --input='["test"]'
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, local, err := getLocalDeps(cmd, deps)
			if err != nil {
				return fmt.Errorf("failed to get local deps: %w", err)
			}

			var input workflowviews.TaskRunInput

			if fileName, err := cmd.Flags().GetString("input-file"); err == nil && fileName != "" {
				content, err := os.ReadFile(fileName)
				if err != nil {
					return fmt.Errorf("failed to read input file: %w", err)
				}
				cmd.Flags().Set("input", string(content))
			}

			err = command.ParseCommand(cmd, args, &input)
			if err != nil {
				return fmt.Errorf("failed to parse input: %w", err)
			}

			if nonInteractive, err := command.NonInteractive(cmd, func() (*workflows.TaskRun, error) {
				taskLoader := deps.WorkflowLoader()
				return taskLoader.CreateTaskRun(cmd.Context(), input)
			}, func(j *workflows.TaskRun) string {
				return text.FormatStringF("Created task run %s for %s", j.Id, input.TaskID)
			}); err != nil {
				return err
			} else if nonInteractive {
				return nil
			}

			flows.NewWorkflow(deps, flows.NewLogFlow(deps, flows.WithLocal(local)), local).TaskRunFlow(cmd.Context(), &input)

			return nil
		},
	}

	cmd.Flags().String(
		"input", "",
		"JSON array input to pass to the task (e.g., '[\"arg1\", \"arg2\"]')",
	)
	cmd.Flags().String("input-file", "", "File containing JSON input to pass to the task")
	cmd.MarkFlagFilename("input-file")

	return cmd
}

func init() {
	taskCmd.PersistentFlags().Bool("local", false, "Run against the server spawned by the task dev command")
	taskCmd.PersistentFlags().Int("port", defaultTaskAPIPort, "Port of the local task server (8120 when not specified)")

	taskDevCmd.Flags().Int("port", defaultTaskAPIPort, "Port of the local task server (8120 when not specified)")

	EarlyAccessCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskDevCmd)
}

type localDeps struct {
	localPort int
	flows.WorkflowDeps
	client *client.ClientWithResponses
}

func newLocalDeps(deps flows.WorkflowDeps, localPort int) (*localDeps, error) {
	client, err := client.NewLocalClient(localPort)
	if err != nil {
		return nil, err
	}
	return &localDeps{client: client, WorkflowDeps: deps, localPort: localPort}, nil
}

func (d *localDeps) TaskRepo() *tasks.Repo {
	return tasks.NewRepo(d.client)
}

func (d *localDeps) LogRepo() *logs.LogRepo {
	return logs.NewLogRepo(d.client, client.LocalConfig(d.localPort))
}

func (d *localDeps) WorkflowLoader() *workflowviews.WorkflowLoader {
	return workflowviews.NewWorkflowLoader(d.TaskRepo(), nil, nil, nil)
}

func (d *localDeps) LogLoader() *views.LogLoader {
	return views.NewLogLoader(d.LogRepo(), nil, nil, nil, nil)
}

func getLocalDeps(cmd *cobra.Command, deps flows.WorkflowDeps) (flows.WorkflowDeps, bool, error) {
	local, err := cmd.Flags().GetBool("local")
	if err != nil {
		return nil, false, fmt.Errorf("failed to get local flag: %w", err)
	}

	localPort, err := cmd.Flags().GetInt("port")
	if err != nil {
		return nil, false, fmt.Errorf("failed to get local port flag: %w", err)
	}

	if local {
		deps, err = newLocalDeps(deps, localPort)
		if err != nil {
			return nil, false, fmt.Errorf("failed to create local deps: %w", err)
		}
	}
	return deps, local, nil
}

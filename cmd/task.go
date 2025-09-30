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
	Short: "Task commands",
}

var taskDevCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start a task in development mode",
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

func NewTaskRunCmd(deps flows.WorkflowDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [taskID]",
		Short: "Run a new task",
		Args:  cobra.MaximumNArgs(1),
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

	cmd.Flags().String("input", "", "JSON input to pass to the task")
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

package cmd

import (
	"fmt"
	"os"

	"github.com/render-oss/cli/pkg/client"
	workflows "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/logs"
	"github.com/render-oss/cli/pkg/tasks"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/flows"
	"github.com/render-oss/cli/pkg/tui/views"
	workflowviews "github.com/render-oss/cli/pkg/tui/views/workflows"
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Manage tasks",
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
  render workflows taskruns start tsk-1234 --input='["arg1", "arg2"]'
  render workflows taskruns start my-workflow/my-task --input='[42, "hello"]'
  render workflows taskruns start my-task --input-file=input.json
  render workflows taskruns start my-task --local --input='["test"]'
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, local, err := getLocalDeps(cmd, deps)
			if err != nil {
				return fmt.Errorf("failed to get local deps: %w", err)
			}

			var input workflowviews.TaskRunInput

			if fileName, err := cmd.Flags().GetString("input-file"); err == nil && fileName != "" {
				fileName, err = command.ExpandPath(fileName)
				if err != nil {
					return fmt.Errorf("failed to resolve input file path: %w", err)
				}
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

			if input.TaskID != "" && cmd.Flags().Changed("input") {
				command.DefaultFormatNonInteractive(cmd)
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

	WorkflowsCmd.AddCommand(taskCmd)
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
	return views.NewLocalLogLoader(d.LogRepo())
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

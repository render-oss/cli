package cmd

import (
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/logs"
	"github.com/render-oss/cli/pkg/tasks"
	"github.com/render-oss/cli/pkg/tui/flows"
	"github.com/render-oss/cli/pkg/tui/views"
	workflowviews "github.com/render-oss/cli/pkg/tui/views/workflows"
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "tasks",
	Short: "List tasks and manage their runs",
	Example: `  # List tasks in a workflow version
  render workflows tasks list wfv-1234

  # Start a task run
  render workflows tasks runs start --task my-task --input='["arg1"]'

  # List task runs for a task
  render workflows tasks runs list --task my-task`,
}

var tasksRunsCmd = &cobra.Command{
	Use:   "runs",
	Short: "Start, list, and inspect task runs",
	Long: `Manage runs of workflow tasks.

A task run represents a single execution of a task with specific input parameters. Use these commands to start new runs, view task run history, inspect details, and cancel in-progress runs.`,
	Example: `  # Start a task run
  render workflows tasks runs start --task my-task --input='["arg1"]'

  # List task runs for a task
  render workflows tasks runs list --task my-task

  # Show details for a task run
  render workflows tasks runs show trn-1234

  # Cancel a task run
  render workflows tasks runs cancel trn-1234`,
}

func init() {
	taskCmd.PersistentFlags().Bool("local", false, "Run against the local workflow development server")
	taskCmd.PersistentFlags().Int("port", defaultTaskAPIPort, "Set the port of the local task server")
	setFlagPlaceholder(taskCmd.PersistentFlags(), "port", "PORT")

	WorkflowsCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(tasksRunsCmd)
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

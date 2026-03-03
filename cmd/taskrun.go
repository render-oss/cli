package cmd

import "github.com/spf13/cobra"

var taskRunCmd = &cobra.Command{
	Use:   "taskruns",
	Short: "View task runs",
	Long: `View task run executions.

A task run represents a single execution of a task with specific input parameters.
Use these commands to view task run history and inspect details.

Available commands:
  list     - List all runs for a task
  show     - Show detailed information about a specific run

To start a new task run, use 'render workflows tasks start'.
`,
}

func init() {
	taskRunCmd.PersistentFlags().Bool("local", false, "Run against the server spawned by the task dev command")
	taskRunCmd.PersistentFlags().Int("port", defaultTaskAPIPort, "Port of the local task server (8120 when not specified)")

	WorkflowsCmd.AddCommand(taskRunCmd)
}

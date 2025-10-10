package cmd

import "github.com/spf13/cobra"

var taskRunCmd = &cobra.Command{
	Use:   "taskruns",
	Short: "Manage task runs",
	Long: `Manage task run executions.

A task run represents a single execution of a task with specific input parameters.
Use these commands to start new task runs, view their history, and inspect details.

Available commands:
  start    - Execute a task with input parameters
  list     - List all runs for a task
  show     - Show detailed information about a specific run
`,
}

func init() {
	taskRunCmd.PersistentFlags().Bool("local", false, "Run against the server spawned by the task dev command")
	taskRunCmd.PersistentFlags().Int("port", defaultTaskAPIPort, "Port of the local task server (8120 when not specified)")

	EarlyAccessCmd.AddCommand(taskRunCmd)
}

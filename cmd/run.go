package cmd

import (
	"github.com/render-oss/cli/pkg/command"
	"github.com/spf13/cobra"
)

var runsCmd = &cobra.Command{
	Use:   "runs",
	Short: "List and inspect workflow task runs",
	Long: `View task run executions.

A task run represents a single execution of a task with specific input parameters. Use these commands to view task run history and inspect details.

To start a new task run, use:
  render workflows tasks start`,
	Example: `  # List task runs for a task
  render workflows runs list tsk-1234

  # Show details for a task run
  render workflows runs show tr-1234
`,
}

func init() {
	runsCmd.PersistentFlags().Bool("local", false, "Run against the local workflow development server")
	runsCmd.PersistentFlags().Int("port", defaultTaskAPIPort, "Set the port of the local task server")
	setAnnotationBestEffort(runsCmd.PersistentFlags(), "port", command.FlagPlaceholderAnnotation, []string{"PORT"})

	WorkflowsCmd.AddCommand(runsCmd)
}

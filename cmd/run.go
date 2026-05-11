package cmd

import (
	"github.com/render-oss/cli/pkg/command"
	"github.com/spf13/cobra"
)

var runsCmd = &cobra.Command{
	Use:        "runs",
	Short:      "List and inspect workflow task runs",
	Hidden:     true,
	Deprecated: "use `render workflows tasks runs` instead.",
	Long: `View task run executions.

A task run represents a single execution of a task with specific input parameters. Use these commands to view task run history and inspect details.

To start a new task run, use either:
  render workflows start
  render workflows tasks runs start`,
}

func init() {
	runsCmd.PersistentFlags().Bool("local", false, "Run against the local workflow development server")
	runsCmd.PersistentFlags().Int("port", defaultTaskAPIPort, "Set the port of the local task server")
	setAnnotationBestEffort(runsCmd.PersistentFlags(), "port", command.FlagPlaceholderAnnotation, []string{"PORT"})

	WorkflowsCmd.AddCommand(runsCmd)
}

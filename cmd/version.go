package cmd

import (
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "versions",
	Short: "List and release workflow versions",
	Example: `  # List versions for a workflow
  render workflows versions list wf-abc123

  # Release a new workflow version
  render workflows versions release wf-abc123`,
}

func init() {
	WorkflowsCmd.AddCommand(versionCmd)
}

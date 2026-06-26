package cmd

import (
	"github.com/render-oss/cli/pkg/tui/flows"
	"github.com/spf13/cobra"
)

// addLocalServerFlags adds the --local and --port flags directly to a command.
// Used by top-level workflow shortcut commands that don't inherit these from
// a parent's PersistentFlags.
func addLocalServerFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("local", false, "Run against the local workflow development server")
	cmd.Flags().Int("port", defaultTaskAPIPort, "Set the port of the local task server")
	setFlagPlaceholder(cmd.Flags(), "port", "PORT")
}

func workflowStartShortcut(deps flows.WorkflowDeps) *cobra.Command {
	cmd := NewRunStartCmd(deps)
	cmd.Use = "start [taskSlug]"
	cmd.Short = "Start a workflow task run (shortcut for `tasks runs start`)"
	cmd.Example = `  # Start a task run by task slug
  render workflows start my-workflow/my-task --input='["arg1"]'

  # Start a task run with --task
  render workflows start --task tsk-1234 --input='["arg1", "arg2"]'

  # Start a task run with input from a file
  render workflows start my-task --input-file=input.json

  # Start against the local workflow development server
  render workflows start my-task --local --input='["test"]'`
	addLocalServerFlags(cmd)
	return cmd
}

func workflowCancelShortcut(deps flows.WorkflowDeps) *cobra.Command {
	cmd := NewRunCancelCmd(deps)
	cmd.Use = "cancel <taskRunID>"
	cmd.Short = "Cancel a workflow task run (shortcut for `tasks runs cancel`)"
	cmd.Example = `  # Cancel a remote task run
  render workflows cancel trn-abc123

  # Cancel a task run in the local dev server
  render workflows cancel --local trn-xyz789`
	addLocalServerFlags(cmd)
	return cmd
}

func deprecatedTaskStartCmd(deps flows.WorkflowDeps) *cobra.Command {
	cmd := NewRunStartCmd(deps)
	cmd.Hidden = true
	cmd.Deprecated = "use `render workflows tasks runs start` instead."
	return cmd
}

func deprecatedRunListCmd(deps flows.WorkflowDeps) *cobra.Command {
	cmd := NewRunListCmd(deps)
	cmd.Hidden = true
	cmd.Deprecated = "use `render workflows tasks runs list` instead."
	return cmd
}

func deprecatedRunDetailsCmd(deps flows.WorkflowDeps) *cobra.Command {
	cmd := NewRunDetailsCmd(deps)
	cmd.Hidden = true
	cmd.Deprecated = "use `render workflows tasks runs show` instead."
	return cmd
}

func deprecatedRunCancelCmd(deps flows.WorkflowDeps) *cobra.Command {
	cmd := NewRunCancelCmd(deps)
	cmd.Hidden = true
	cmd.Deprecated = "use `render workflows tasks runs cancel` instead."
	return cmd
}

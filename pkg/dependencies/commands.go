package dependencies

import "github.com/spf13/cobra"

type WorkflowCommands struct {
	VersionCmd        *cobra.Command
	VersionListCmd    *cobra.Command
	VersionReleaseCmd *cobra.Command
	TaskListCmd       *cobra.Command
	TaskRunCmd        *cobra.Command
	TaskRunListCmd    *cobra.Command
	TaskRunDetailsCmd *cobra.Command
	WorkflowListCmd   *cobra.Command
}

type LogsCommands struct {
	LogsCmd *cobra.Command
}

type WorkspaceCommands struct {
	WorkspaceSetCmd *cobra.Command
}

type Commands struct {
	Workflow  *WorkflowCommands
	Logs      *LogsCommands
	Workspace *WorkspaceCommands
}

func NewCommands(workflow *WorkflowCommands) *Commands {
	return &Commands{Workflow: workflow}
}

func (c *Commands) ListVersions() *cobra.Command {
	return c.Workflow.VersionListCmd
}

func (c *Commands) ReleaseVersion() *cobra.Command {
	return c.Workflow.VersionReleaseCmd
}

func (c *Commands) Version() *cobra.Command {
	return c.Workflow.VersionCmd
}

func (c *Commands) ListTask() *cobra.Command {
	return c.Workflow.TaskListCmd
}

func (c *Commands) RunTask() *cobra.Command {
	return c.Workflow.TaskRunCmd
}

func (c *Commands) ListTaskRuns() *cobra.Command {
	return c.Workflow.TaskRunListCmd
}

func (c *Commands) ListWorkflow() *cobra.Command {
	return c.Workflow.WorkflowListCmd
}

func (c *Commands) LogsCmd() *cobra.Command {
	return c.Logs.LogsCmd
}

func (c *Commands) WorkspaceSetCmd() *cobra.Command {
	return c.Workspace.WorkspaceSetCmd
}

func (c *Commands) TaskRunDetailsCmd() *cobra.Command {
	return c.Workflow.TaskRunDetailsCmd
}

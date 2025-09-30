package flows

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
	"github.com/spf13/cobra"
)

type WorkspaceFlowDeps interface {
	Stack() *tui.StackModel
	WorkspaceSetCmd() *cobra.Command
}

type WorkspaceFlow struct {
	deps WorkspaceFlowDeps
}

func NewWorkspaceFlow(deps WorkspaceFlowDeps) *WorkspaceFlow {
	return &WorkspaceFlow{deps: deps}
}

func (f *WorkspaceFlow) WorkspaceSetFlow(ctx context.Context, input views.ListWorkspaceInput) tea.Cmd {
	return command.AddToStack(f.deps.Stack(), f.deps.WorkspaceSetCmd(), "Set Workspace", &input, views.NewWorkspaceView(ctx, input))
}

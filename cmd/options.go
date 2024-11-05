package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/spf13/cobra"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/renderinc/render-cli/pkg/tui/views"
)

func WithWorkspaceSelection(ctx context.Context) tui.CustomOption {
	return tui.CustomOption{
		Key:   "w",
		Title: "Change Workspace",
		Function: func(row btable.Row) tea.Cmd {
			return InteractiveWorkspaceSet(ctx, views.ListWorkspaceInput{})
		},
	}
}

type ProjectHandler func(ctx context.Context, project *client.Project) tea.Cmd

func WithProjectFilter(ctx context.Context, cmd *cobra.Command, breadcrumb string, in any, h ProjectHandler) tui.CustomOption {
	return tui.CustomOption{
		Key:   "f",
		Title: "Filter by Project",
		Function: func(row btable.Row) tea.Cmd {
			return command.AddToStackFunc(ctx, cmd, breadcrumb, in,
				views.NewProjectFilterView(ctx, h),
			)
		},
	}
}

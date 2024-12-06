package cmd

import (
	"context"
	"fmt"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/tui"
	"github.com/renderinc/cli/pkg/tui/views"
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

func WithCopyID(ctx context.Context, cmd *cobra.Command) tui.CustomOption {
	return tui.CustomOption{
		Key:   "c",
		Title: "Copy ID",
		Function: func(row btable.Row) tea.Cmd {
			return func() tea.Msg {
				id, ok := row.Data["ID"]
				if !ok {
					return nil
				}

				idstr, ok := id.(string)
				if !ok {
					return nil
				}

				err := clipboard.WriteAll(idstr)
				if err != nil {
					return command.AddErrToStack(ctx, cmd, fmt.Errorf("could not copy to clipboard: %w", err))
				}
				return nil
			}
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

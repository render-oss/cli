package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/tui"
)

type ProjectFilterView struct {
	table *tui.Table[*client.Project]
}

func NewProjectFilterView(ctx context.Context, onSelect func(context.Context, *client.Project) tea.Cmd) *ProjectList {
	saveProjectFilter := func(row btable.Row) tea.Cmd {
		p, ok := row.Data["project"].(*client.Project)
		if !ok {
			return func() tea.Msg {
				return tui.ErrorMsg{Err: fmt.Errorf("failed to get project from row")}
			}
		}

		if err := config.SetProjectFilter(p.Id, p.Name); err != nil {
			return func() tea.Msg {
				return tui.ErrorMsg{Err: fmt.Errorf("failed to save project filter: %w", err)}
			}
		}
		return onSelect(ctx, p)
	}

	clearProjectFilter := func(row btable.Row) tea.Cmd {
		if err := config.ClearProjectFilter(); err != nil {
			return func() tea.Msg {
				return tui.ErrorMsg{Err: fmt.Errorf("failed to clear project filter: %w", err)}
			}
		}
		return onSelect(ctx, nil)
	}

	customOptions := []tui.CustomOption{
		{
			Key:      "s",
			Title:    "Save as default filter",
			Function: saveProjectFilter,
		},
		{
			Key:      "x",
			Title:    "Clear filter",
			Function: clearProjectFilter,
		},
	}

	return NewProjectList(
		ctx,
		onSelect,
		tui.WithCustomOptions[*client.Project](customOptions),
	)
}

func (v *ProjectFilterView) Init() tea.Cmd {
	return v.table.Init()
}

func (v *ProjectFilterView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v.table.Update(msg)
}

func (v *ProjectFilterView) View() string {
	return v.table.View()
}

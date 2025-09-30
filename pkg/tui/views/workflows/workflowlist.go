package workflows

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/resource"
	resourcetui "github.com/render-oss/cli/pkg/resource/tui"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/workflow"
)

type WorkflowList struct {
	table *tui.Table[*workflow.Model]
}

type OnSelectFuncT[T any] func(context.Context, T) tea.Cmd

func NewWorkflowList(ctx context.Context, workflowLoader *WorkflowLoader, in WorkflowInput, selectFunc OnSelectFuncT[resource.Resource], opts ...tui.TableOption[*workflow.Model]) *WorkflowList {
	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		r, ok := rows[0].Data["resource"].(resource.Resource)
		if !ok {
			return nil
		}

		return selectFunc(ctx, r)
	}

	return &WorkflowList{
		table: tui.NewTable(
			resourcetui.ColumnsForResources(),
			command.LoadCmd(ctx, workflowLoader.ListWorkflows, in),
			func(s *workflow.Model) btable.Row {
				return resourcetui.RowForResource(s)
			},
			onSelect,
			opts...,
		),
	}
}

type WorkflowInput struct {
	Project        *client.Project
	EnvironmentIDs []string
}

func (pl *WorkflowList) Init() tea.Cmd {
	return pl.table.Init()
}

func (pl *WorkflowList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return pl.table.Update(msg)
}

func (pl *WorkflowList) View() string {
	return pl.table.View()
}

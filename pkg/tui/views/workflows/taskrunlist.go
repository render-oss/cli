package workflows

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"

	wfclient "github.com/render-oss/cli/v2/pkg/client/workflows"
	"github.com/render-oss/cli/v2/pkg/command"
	"github.com/render-oss/cli/v2/pkg/taskrun"
	"github.com/render-oss/cli/v2/pkg/tui"
)

type TaskRunListView struct {
	table *tui.Table[*wfclient.TaskRun]
}

func NewTaskRunListView(ctx context.Context, workflowLoader *WorkflowLoader, input TaskRunListInput, generateCommands func(*wfclient.TaskRun) tea.Cmd) *TaskRunListView {
	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		tr, ok := rows[0].Data["taskRun"].(*wfclient.TaskRun)
		if !ok {
			return nil
		}

		return generateCommands(tr)
	}

	return &TaskRunListView{
		table: tui.NewTable(
			taskrun.Columns(),
			command.LoadCmd(ctx, workflowLoader.LoadAllTaskRuns, input),
			func(tr *wfclient.TaskRun) btable.Row {
				return taskrun.TableRow(tr)
			},
			onSelect,
		),
	}
}

func (v *TaskRunListView) Init() tea.Cmd {
	return v.table.Init()
}

func (v *TaskRunListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := v.table.Update(msg)
	return v, cmd
}

func (v *TaskRunListView) View() string {
	return v.table.View()
}

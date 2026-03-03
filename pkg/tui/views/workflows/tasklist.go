package workflows

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/task"
	"github.com/render-oss/cli/pkg/tui"
)

type TaskListView struct {
	table *tui.Table[*wfclient.Task]
}

func NewTaskListView(ctx context.Context, workflowLoader *WorkflowLoader, input TaskListInput, generateCommands func(*wfclient.Task) tea.Cmd) *TaskListView {
	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		t, ok := rows[0].Data["task"].(*wfclient.Task)
		if !ok {
			return nil
		}

		return generateCommands(t)
	}

	return &TaskListView{
		table: tui.NewTable(
			task.Columns(),
			command.LoadCmd(ctx, workflowLoader.LoadAllTasks, input),
			func(t *wfclient.Task) btable.Row {
				return task.TableRow(t)
			},
			onSelect,
		),
	}
}

func (v *TaskListView) Init() tea.Cmd {
	return v.table.Init()
}

func (v *TaskListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := v.table.Update(msg)
	return v, cmd
}

func (v *TaskListView) View() string {
	return v.table.View()
}

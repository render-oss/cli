package workflows

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/taskrun"
	"github.com/render-oss/cli/pkg/tui"
)

type TaskRunListView struct {
	list *tui.List[*wfclient.TaskRun]
}

func NewTaskRunListView(ctx context.Context, workflowLoader *WorkflowLoader, input TaskRunListInput, generateCommands func(*wfclient.TaskRun) tea.Cmd) *TaskRunListView {
	onSelect := func(selectedItem tui.ListItem) tea.Cmd {
		selectedTaskRun := selectedItem.(taskrun.ListItem).TaskRun()
		return generateCommands(selectedTaskRun)
	}

	list := tui.NewList(
		"",
		command.PaginatedLoadCmd(ctx, workflowLoader.LoadTaskRunList, input),
		func(tr *wfclient.TaskRun) tui.ListItem {
			return taskrun.NewListItem(tr)
		},
		tui.WithOnSelect[*wfclient.TaskRun](onSelect),
	)

	return &TaskRunListView{
		list: list,
	}
}

func (v *TaskRunListView) Init() tea.Cmd {
	return v.list.Init()
}

func (v *TaskRunListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := v.list.Update(msg)
	return v, cmd
}

func (v *TaskRunListView) View() string {
	return v.list.View()
}

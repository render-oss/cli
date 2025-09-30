package workflows

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/task"
	"github.com/render-oss/cli/pkg/tasks"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/version"
)

type TaskListDeps interface {
	TaskRepo() *tasks.Repo
	WorkflowVersionRepo() *version.Repo
}

type TaskListView struct {
	list *tui.List[*wfclient.Task]
}

func NewTaskListView(ctx context.Context, workflowLoader *WorkflowLoader, input TaskListInput, generateCommands func(*wfclient.Task) tea.Cmd) *TaskListView {
	onSelect := func(selectedItem tui.ListItem) tea.Cmd {
		selectedTask := selectedItem.(task.ListItem).Task()
		return generateCommands(selectedTask)
	}

	return &TaskListView{
		list: tui.NewList(
			"",
			command.PaginatedLoadCmd(ctx, workflowLoader.LoadTaskList, input),
			func(t *wfclient.Task) tui.ListItem {
				return task.NewListItem(t)
			},
			tui.WithOnSelect[*wfclient.Task](onSelect),
		),
	}
}

func (v *TaskListView) Init() tea.Cmd {
	return v.list.Init()
}

func (v *TaskListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := v.list.Update(msg)
	return v, cmd
}

func (v *TaskListView) View() string {
	return v.list.View()
}

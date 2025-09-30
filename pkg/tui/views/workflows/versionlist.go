package workflows

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/version"
)

type VersionListView struct {
	list *tui.List[*wfclient.WorkflowVersion]
}

func NewVersionListView(ctx context.Context, workflowLoader *WorkflowLoader, input VersionListInput, generateCommands func(*wfclient.WorkflowVersion) tea.Cmd) *VersionListView {
	onSelect := func(selectedItem tui.ListItem) tea.Cmd {
		selectedVersion := selectedItem.(version.ListItem).Version()
		return generateCommands(selectedVersion)
	}

	list := tui.NewList(
		"",
		command.PaginatedLoadCmd(ctx, workflowLoader.LoadVersionList, input),
		func(v *wfclient.WorkflowVersion) tui.ListItem {
			return version.NewListItem(v)
		},
		tui.WithOnSelect[*wfclient.WorkflowVersion](onSelect),
	)

	return &VersionListView{
		list: list,
	}
}

func (v *VersionListView) Init() tea.Cmd {
	return v.list.Init()
}

func (v *VersionListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := v.list.Update(msg)
	return v, cmd
}

func (v *VersionListView) View() string {
	return v.list.View()
}

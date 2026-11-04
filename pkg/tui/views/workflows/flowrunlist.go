package workflows

import (
	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/flowrun"
	"github.com/render-oss/cli/pkg/tui"
)

type FlowRunListView struct {
	table *tui.Table[*wfclient.FlowRun]
}

func NewFlowRunListView(loadRuns tui.TypedCmd[[]*wfclient.FlowRun], generateCommands func(*wfclient.FlowRun) tea.Cmd) *FlowRunListView {
	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		run, ok := rows[0].Data["flowRun"].(*wfclient.FlowRun)
		if !ok {
			return nil
		}

		return generateCommands(run)
	}

	return &FlowRunListView{
		table: tui.NewTable(
			flowrun.RunColumns(),
			loadRuns,
			flowrun.RunTableRow,
			onSelect,
		),
	}
}

func (v *FlowRunListView) Init() tea.Cmd {
	return v.table.Init()
}

func (v *FlowRunListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := v.table.Update(msg)
	return v, cmd
}

func (v *FlowRunListView) View() string {
	return v.table.View()
}

package workflows

import (
	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/flowrun"
	"github.com/render-oss/cli/pkg/tui"
)

type FlowListView struct {
	table *tui.Table[*wfclient.Flow]
}

func NewFlowListView(loadFlows tui.TypedCmd[[]*wfclient.Flow], generateCommands func(*wfclient.Flow) tea.Cmd) *FlowListView {
	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		f, ok := rows[0].Data["flow"].(*wfclient.Flow)
		if !ok {
			return nil
		}

		return generateCommands(f)
	}

	return &FlowListView{
		table: tui.NewTable(
			flowrun.FlowColumns(),
			loadFlows,
			flowrun.FlowTableRow,
			onSelect,
		),
	}
}

func (v *FlowListView) Init() tea.Cmd {
	return v.table.Init()
}

func (v *FlowListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := v.table.Update(msg)
	return v, cmd
}

func (v *FlowListView) View() string {
	return v.table.View()
}

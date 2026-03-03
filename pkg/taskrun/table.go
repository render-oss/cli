package taskrun

import (
	"github.com/evertras/bubble-table/table"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
)

func Columns() []table.Column {
	return []table.Column{
		table.NewColumn("ID", "ID", 30).WithFiltered(true),
		table.NewFlexColumn("Status", "Status", 2).WithFiltered(true),
		table.NewFlexColumn("Started", "Started", 3),
		table.NewFlexColumn("Completed", "Completed", 3),
		table.NewFlexColumn("Duration", "Duration", 2),
	}
}

func TableRow(tr *wfclient.TaskRun) table.Row {
	var started, completed, duration string

	if tr.StartedAt != nil {
		started = pointers.TimeValue(tr.StartedAt)

		if tr.CompletedAt != nil {
			completed = pointers.TimeValue(tr.CompletedAt)
			duration = tr.CompletedAt.Sub(*tr.StartedAt).String()
		}
	}

	return table.NewRow(table.RowData{
		"ID":        tr.Id,
		"Status":    table.NewStyledCell(string(tr.Status), statusWithStyle(tr.Status)),
		"Started":   started,
		"Completed": completed,
		"Duration":  duration,
		"taskRun":   tr,
	})
}

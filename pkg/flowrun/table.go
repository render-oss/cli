package flowrun

import (
	"github.com/evertras/bubble-table/table"

	workflows "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
)

// FlowColumns returns the columns for the interactive flow list table.
func FlowColumns() []table.Column {
	return []table.Column{
		table.NewFlexColumn("Name", "Name", 4).WithFiltered(true),
		table.NewColumn("ID", "ID", 30).WithFiltered(true),
		table.NewFlexColumn("Created", "Created", 3),
	}
}

// FlowTableRow returns the interactive table row for a single flow.
func FlowTableRow(f *workflows.Flow) table.Row {
	return table.NewRow(table.RowData{
		"Name":    f.Name,
		"ID":      f.Id,
		"Created": pointers.TimeValue(&f.CreatedAt),
		"flow":    f,
	})
}

// RunColumns returns the columns for the interactive flow run list table.
func RunColumns() []table.Column {
	return []table.Column{
		table.NewColumn("ID", "ID", 30).WithFiltered(true),
		table.NewFlexColumn("Status", "Status", 2).WithFiltered(true),
		table.NewFlexColumn("Started", "Started", 3),
		table.NewFlexColumn("Completed", "Completed", 3),
		table.NewFlexColumn("Duration", "Duration", 2),
	}
}

// RunTableRow returns the interactive table row for a single flow run.
func RunTableRow(run *workflows.FlowRun) table.Row {
	var started, completed, duration string
	if run.StartedAt != nil {
		started = pointers.TimeValue(run.StartedAt)
		if run.CompletedAt != nil {
			completed = pointers.TimeValue(run.CompletedAt)
			duration = run.CompletedAt.Sub(*run.StartedAt).String()
		}
	}

	return table.NewRow(table.RowData{
		"ID":        run.Id,
		"Status":    table.NewStyledCell(string(run.Status), runStatusStyle(run.Status)),
		"Started":   started,
		"Completed": completed,
		"Duration":  duration,
		"flowRun":   run,
	})
}

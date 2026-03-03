package task

import (
	"github.com/evertras/bubble-table/table"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
)

func Columns() []table.Column {
	return []table.Column{
		table.NewFlexColumn("Name", "Name", 4).WithFiltered(true),
		table.NewColumn("ID", "ID", 30).WithFiltered(true),
		table.NewFlexColumn("Created", "Created", 3),
	}
}

func TableRow(t *wfclient.Task) table.Row {
	return table.NewRow(table.RowData{
		"Name":    t.Name,
		"ID":      t.Id,
		"Created": pointers.TimeValue(&t.CreatedAt),
		"task":    t,
	})
}

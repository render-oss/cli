package tui

import (
	"github.com/evertras/bubble-table/table"

	"github.com/renderinc/cli/pkg/resource"
)

func Columns() []table.Column {
	return []table.Column{
		table.NewFlexColumn("Name", "Name", 4).WithFiltered(true),
		table.NewFlexColumn("Project", "Project", 2).WithFiltered(true),
		table.NewFlexColumn("Environment", "Environment", 2).WithFiltered(true),
		table.NewColumn("ID", "ID", 27).WithFiltered(true),
	}
}

func Row(r resource.Resource) table.Row {
	return table.NewRow(table.RowData{
		"ID":          r.ID(),
		"Project":     r.ProjectName(),
		"Environment": r.EnvironmentName(),
		"Name":        r.Name(),
		"resource":    r, // this will be hidden in the UI, but will be used to get the resource when selected
	})
}

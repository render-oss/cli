package tui

import (
	"github.com/evertras/bubble-table/table"

	"github.com/render-oss/cli/pkg/resource"
)

func ColumnsForResources() []table.Column {
	return []table.Column{
		table.NewFlexColumn("Name", "Name", 8).WithFiltered(true),
		table.NewFlexColumn("Project", "Project", 4).WithFiltered(true),
		table.NewFlexColumn("Environment", "Environment", 4).WithFiltered(true),
		table.NewFlexColumn("Type", "Type", 3).WithFiltered(true),
		table.NewColumn("ID", "ID", 30).WithFiltered(true),
	}
}

func RowForResource(r resource.Resource) table.Row {
	return table.NewRow(table.RowData{
		"ID":          r.ID(),
		"Type":        r.Type(),
		"Project":     r.ProjectName(),
		"Environment": r.EnvironmentName(),
		"Name":        r.Name(),
		"resource":    r, // this will be hidden in the UI, but will be used to get the resource when selected
	})
}

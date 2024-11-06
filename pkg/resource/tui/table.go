package tui

import (
	"github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/resource"
)

func ColumnsForResources() []table.Column {
	return []table.Column{
		table.NewFlexColumn("Name", "Name", 40).WithFiltered(true),
		table.NewFlexColumn("Project", "Project", 15).WithFiltered(true),
		table.NewFlexColumn("Environment", "Environment", 20).WithFiltered(true),
		table.NewFlexColumn("Type", "Type", 12).WithFiltered(true),
		table.NewColumn("ID", "ID", 27).WithFiltered(true),
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

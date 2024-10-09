package resource

import (
	"github.com/evertras/bubble-table/table"
)


func ColumnsForResources() []table.Column {
	return []table.Column{
		table.NewColumn("ID", "ID", 25).WithFiltered(true),
		table.NewColumn("Type", "Type", 12).WithFiltered(true),
		table.NewColumn("Project", "Project", 15).WithFiltered(true),
		table.NewColumn("Environment", "Environment", 20).WithFiltered(true),
		table.NewColumn("Name", "Name", 40).WithFiltered(true),
	}
}

func RowsForResources(resources []Resource) ([]table.Row) {
	var rows []table.Row
	for _, r := range resources {
		rows = append(rows, table.NewRow(table.RowData{
			"ID":          r.ID(),
			"Type":        r.Type(),
			"Project":     r.ProjectName(),
			"Environment": r.EnvironmentName(),
			"Name":        r.Name(),
			"resource":    r, // this will be hidden in the UI, but will be used to get the resource when selected
		}))
	}
	
	return rows
}

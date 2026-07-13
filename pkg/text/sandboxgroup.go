package text

import (
	"fmt"

	"github.com/jedib0t/go-pretty/table"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/utils"
)

func SandboxGroupTable(groups []*sandboxesclient.SandboxGroup) string {
	t := newTable()
	t.AppendHeader(table.Row{"ID", "Name", "Region", "Default", "Environment", "Age"})
	for _, g := range groups {
		env := "-"
		if g.EnvironmentId != nil && *g.EnvironmentId != "" {
			env = *g.EnvironmentId
		}
		t.AppendRow(table.Row{
			g.Id,
			g.Name,
			g.Region,
			fmt.Sprintf("%t", g.IsDefault),
			env,
			utils.FormatDuration(g.CreatedAt),
		})
	}
	return FormatString(t.Render())
}

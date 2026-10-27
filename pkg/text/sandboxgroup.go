package text

import (
	"fmt"
	"strings"
	"time"

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

func SandboxGroupDetail(g *sandboxesclient.SandboxGroup) string {
	env := "-"
	if g.EnvironmentId != nil && *g.EnvironmentId != "" {
		env = *g.EnvironmentId
	}
	lines := []string{
		fmt.Sprintf("ID:                %s", g.Id),
		fmt.Sprintf("Name:              %s", g.Name),
		fmt.Sprintf("Region:            %s", g.Region),
		fmt.Sprintf("Default:           %t", g.IsDefault),
		fmt.Sprintf("Concurrency limit: %d", g.ConcurrencyLimit),
		fmt.Sprintf("Environment:       %s", env),
		fmt.Sprintf("Created:           %s", g.CreatedAt.UTC().Format(time.RFC3339)),
	}
	return FormatString(strings.Join(lines, "\n"))
}

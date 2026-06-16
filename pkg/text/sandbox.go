package text

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/table"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/utils"
)

func SandboxTable(sandboxes []*sandboxclient.Sandbox) string {
	t := newTable()
	t.AppendHeader(table.Row{"ID", "Status", "Plan", "Region", "Age"})
	for _, s := range sandboxes {
		t.AppendRow(table.Row{
			s.Id,
			s.Status,
			s.Plan,
			s.Region,
			utils.FormatDuration(s.CreatedAt),
		})
	}
	return FormatString(t.Render())
}

func SandboxDetail(sandbox *sandboxclient.Sandbox) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("ID:      %s", sandbox.Id))
	lines = append(lines, fmt.Sprintf("Status:  %s", sandbox.Status))
	lines = append(lines, fmt.Sprintf("Plan:    %s", sandbox.Plan))
	lines = append(lines, fmt.Sprintf("Region:  %s", sandbox.Region))
	lines = append(lines, fmt.Sprintf("Network: default=%s", sandbox.NetworkPolicy.Default))

	lines = append(lines, fmt.Sprintf("Timeout: %ds", sandbox.TimeoutSeconds))

	return FormatString(strings.Join(lines, "\n"))
}

func SandboxTerminated(id string) string {
	return FormatStringF("Sandbox %s terminated", id)
}

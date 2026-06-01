package text

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/table"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/keyvalue"
)

func KeyValueTable(v []*keyvalue.Model) string {
	t := newTable()
	t.AppendHeader(table.Row{"Name", "Project", "Environment", "Plan", "Region", "Status", "ID"})
	for _, m := range v {
		t.AppendRow(table.Row{
			m.Name(),
			m.ProjectName(),
			m.EnvironmentName(),
			string(m.KeyValue.Plan),
			string(m.KeyValue.Region),
			string(m.KeyValue.Status),
			m.ID(),
		})
	}
	return FormatString(t.Render())
}

// KeyValueDetail formats a KV instance detail for text output.
// Does NOT include an action prefix (e.g., "Created" or "Updated") — callers should prepend
// their own action prefix in the formatText closure passed to command.NonInteractive.
func KeyValueDetail(kv *client.KeyValueDetail) string {
	lines := []string{
		fmt.Sprintf("Name: %s", kv.Name),
		fmt.Sprintf("ID: %s", kv.Id),
		fmt.Sprintf("Plan: %s", string(kv.Plan)),
		fmt.Sprintf("Region: %s", string(kv.Region)),
		fmt.Sprintf("Status: %s", string(kv.Status)),
	}
	if kv.Options.MaxmemoryPolicy != nil {
		lines = append(lines, fmt.Sprintf("Memory policy: %s", *kv.Options.MaxmemoryPolicy))
	}
	lines = append(lines, ipAllowListBlock(kv.IpAllowList))
	return strings.Join(lines, "\n")
}

func KeyValueGetDetail(kv *client.KeyValueDetail, conn *client.KeyValueConnectionInfo) string {
	detail := KeyValueDetail(kv)
	if len(kv.IpAllowList) == 0 {
		detail = strings.Replace(detail, "IP allow-list: (empty)", "IP allow-list: (empty, external connections are blocked)", 1)
	}
	if conn == nil {
		return detail
	}
	return strings.Join([]string{
		detail,
		"",
		fmt.Sprintf("CLI:      %s", conn.CliCommand),
		fmt.Sprintf("Internal: %s", conn.InternalConnectionString),
		fmt.Sprintf("External: %s", conn.ExternalConnectionString),
	}, "\n")
}

// KeyValueUpdateDiff renders the user-visible changes between before and after
// snapshots of a Key Value instance, showing only the fields that actually
// changed. Returns an empty string when nothing changed (the cmd layer can
// then surface a "no changes" message).
func KeyValueUpdateDiff(before, after *client.KeyValueDetail) string {
	var lines []string

	if before.Name != after.Name {
		lines = append(lines, fmt.Sprintf("  Name:          %s → %s", before.Name, after.Name))
	}
	if before.Plan != after.Plan {
		lines = append(lines, fmt.Sprintf("  Plan:          %s → %s", before.Plan, after.Plan))
	}
	if beforePolicy, afterPolicy := memoryPolicyLabel(before), memoryPolicyLabel(after); beforePolicy != afterPolicy {
		lines = append(lines, fmt.Sprintf("  Memory policy: %s → %s", beforePolicy, afterPolicy))
	}
	if !ipAllowListEqual(before.IpAllowList, after.IpAllowList) {
		lines = append(lines, fmt.Sprintf("  IP allow-list: %s → %s",
			ipAllowListLabel(before.IpAllowList), ipAllowListLabel(after.IpAllowList)))
	}

	return strings.Join(lines, "\n")
}

func memoryPolicyLabel(kv *client.KeyValueDetail) string {
	if kv.Options.MaxmemoryPolicy == nil {
		return "(none)"
	}
	return *kv.Options.MaxmemoryPolicy
}

package text

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/table"

	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/pointers"
	rstrings "github.com/render-oss/cli/pkg/strings"
)

func KeyValueTable(v []keyvalue.KeyValueOut) string {
	t := newTable()
	t.AppendHeader(table.Row{"Name", "Project", "Environment", "Plan", "Region", "Status", "ID"})
	for _, kv := range v {
		t.AppendRow(table.Row{
			kv.Name,
			kv.ProjectName,
			kv.EnvironmentName,
			string(kv.Plan),
			string(kv.Region),
			string(kv.Status),
			kv.ID,
		})
	}
	return FormatString(t.Render())
}

func KeyValueDetail(kv *keyvalue.KeyValueOut) string {
	lines := []string{
		fmt.Sprintf("Name: %s", kv.Name),
		fmt.Sprintf("ID: %s", kv.ID),
	}
	if line := workspaceLine(kv.WorkspaceName, kv.OwnerID); line != "" {
		lines = append(lines, line)
	}
	if label := rstrings.ResourceLabel(kv.ProjectName, pointers.StringValue(kv.ProjectID)); label != "" {
		lines = append(lines, fmt.Sprintf("Project: %s", label))
	}
	if label := rstrings.ResourceLabel(kv.EnvironmentName, pointers.StringValue(kv.EnvironmentID)); label != "" {
		lines = append(lines, fmt.Sprintf("Environment: %s", label))
	}
	lines = append(lines,
		fmt.Sprintf("Plan: %s", string(kv.Plan)),
		fmt.Sprintf("Region: %s", string(kv.Region)),
		fmt.Sprintf("Status: %s", string(kv.Status)),
	)
	if kv.MaxmemoryPolicy != nil {
		lines = append(lines, fmt.Sprintf("Memory policy: %s", *kv.MaxmemoryPolicy))
	}
	detail := strings.Join(append(lines, ipAllowListBlock(kv.IPAllowList)), "\n")
	if len(kv.IPAllowList) == 0 {
		detail = strings.Replace(detail, "IP allow-list: (empty)", "IP allow-list: (empty, external connections are blocked)", 1)
	}
	if kv.ConnectionInfo == nil {
		return detail
	}
	return strings.Join([]string{
		detail,
		"",
		fmt.Sprintf("CLI:      %s", kv.ConnectionInfo.CliCommand),
		fmt.Sprintf("Internal: %s", kv.ConnectionInfo.InternalConnectionString),
		fmt.Sprintf("External: %s", kv.ConnectionInfo.ExternalConnectionString),
	}, "\n")
}

// KeyValueUpdateDiff renders the user-visible changes from the public update
// diff contract. Returns an empty string when nothing changed (the cmd layer
// can then surface a "no changes" message).
func KeyValueUpdateDiff(diff keyvalue.KeyValueUpdateDiff) string {
	var lines []string

	if diff.Name != nil {
		lines = append(lines, fmt.Sprintf("  Name:          %s → %s", diff.Name.Before, diff.Name.After))
	}
	if diff.Plan != nil {
		lines = append(lines, fmt.Sprintf("  Plan:          %s → %s", diff.Plan.Before, diff.Plan.After))
	}
	if diff.MaxmemoryPolicy != nil {
		lines = append(lines, fmt.Sprintf("  Memory policy: %s → %s",
			memoryPolicyDiffLabel(diff.MaxmemoryPolicy.Before),
			memoryPolicyDiffLabel(diff.MaxmemoryPolicy.After)))
	}
	if diff.IPAllowList != nil {
		lines = append(lines, fmt.Sprintf("  IP allow-list: %s → %s",
			ipAllowListLabel(diff.IPAllowList.Before),
			ipAllowListLabel(diff.IPAllowList.After)))
	}

	return strings.Join(lines, "\n")
}

func memoryPolicyDiffLabel(policy *string) string {
	if policy == nil {
		return "(none)"
	}
	return *policy
}

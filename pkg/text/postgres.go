package text

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/table"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/postgres"
)

func PostgresTable(v []postgres.PostgresListItemOut) string {
	t := newTable()
	t.AppendHeader(table.Row{"Name", "Project", "Environment", "Plan", "Region", "Status", "ID"})
	if len(v) == 0 {
		t.SetCaption("No Postgres databases found.")
	}
	for _, pg := range v {
		t.AppendRow(table.Row{
			pg.Name,
			pg.ProjectName,
			pg.EnvironmentName,
			string(pg.Plan),
			string(pg.Region),
			string(pg.Status),
			pg.Id,
		})
	}
	return FormatString(t.Render())
}

// PostgresDetail formats a Postgres instance detail for text output.
// Does NOT include an action prefix (e.g., "Created") — callers should prepend
// their own action prefix in the formatText closure passed to command.NonInteractive.
func PostgresDetail(pg *postgres.PostgresOut) string {
	lines := []string{
		fmt.Sprintf("Name: %s", pg.Name),
		fmt.Sprintf("ID: %s", pg.Id),
		fmt.Sprintf("Plan: %s", string(pg.Plan)),
		fmt.Sprintf("Version: %s", string(pg.Version)),
		fmt.Sprintf("Region: %s", string(pg.Region)),
		fmt.Sprintf("Status: %s", string(pg.Status)),
		fmt.Sprintf("Database: %s", pg.DatabaseName),
		fmt.Sprintf("User: %s", pg.DatabaseUser),
	}
	if pg.DiskSizeGB != nil {
		lines = append(lines, fmt.Sprintf("Disk size: %d GB", *pg.DiskSizeGB))
	}
	lines = append(lines, fmt.Sprintf("Disk autoscaling: %s", boolLabel(pg.DiskAutoscalingEnabled)))
	lines = append(lines, fmt.Sprintf("High availability: %s", boolLabel(pg.HighAvailabilityEnabled)))
	if pg.EnvironmentId != nil {
		lines = append(lines, fmt.Sprintf("Environment ID: %s", *pg.EnvironmentId))
	}
	lines = append(lines, fmt.Sprintf("Dashboard: %s", pg.DashboardUrl))
	if block := readReplicasBlock(pg.ReadReplicas); block != "" {
		lines = append(lines, block)
	}
	lines = append(lines, ipAllowListBlock(pg.IpAllowList))
	return strings.Join(lines, "\n")
}

func PostgresGetDetail(pg *postgres.PostgresOut) string {
	detail := PostgresDetail(pg)
	if pg.ConnectionInfo == nil {
		return detail
	}
	conn := pg.ConnectionInfo
	return strings.Join([]string{
		detail,
		"",
		fmt.Sprintf("PSQL:     %s", conn.PsqlCommand),
		fmt.Sprintf("Internal: %s", conn.InternalConnectionString),
		fmt.Sprintf("External: %s", conn.ExternalConnectionString),
		fmt.Sprintf("Password: %s", conn.Password),
	}, "\n")
}

// readReplicasBlock renders the read-replica list as a header line followed by
// "  - <name> (<id>)" entries. Returns an empty string when no replicas exist
// — callers should skip the block entirely in that case.
func readReplicasBlock(replicas client.ReadReplicas) string {
	if len(replicas) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Read replicas:")
	for _, r := range replicas {
		fmt.Fprintf(&b, "\n  - %s (%s)", r.Name, r.Id)
	}
	return b.String()
}

func boolLabel(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}

// diskSizeLabel renders a *int disk size for display, returning "(unset)" when nil.
func diskSizeLabel(gb *int) string {
	if gb == nil {
		return "(unset)"
	}
	return fmt.Sprintf("%d GB", *gb)
}

// PostgresUpdateDiff renders the user-visible changes from the public update
// diff contract. Returns an empty string when nothing changed (the cmd layer
// can then surface a "no changes" message).
//
// Label column is padded to 20 characters so the arrows align:
//
//	"  High availability: disabled → enabled"
func PostgresUpdateDiff(diff postgres.PostgresUpdateDiff) string {
	var lines []string

	if diff.Name != nil {
		lines = append(lines, fmt.Sprintf("  %-20s%s → %s", "Name:", diff.Name.Before, diff.Name.After))
	}
	if diff.Plan != nil {
		lines = append(lines, fmt.Sprintf("  %-20s%s → %s", "Plan:", string(diff.Plan.Before), string(diff.Plan.After)))
	}
	if diff.DiskSizeGB != nil {
		lines = append(lines, fmt.Sprintf("  %-20s%s → %s", "Disk size:", diskSizeLabel(diff.DiskSizeGB.Before), diskSizeLabel(diff.DiskSizeGB.After)))
	}
	if diff.DiskAutoscalingEnabled != nil {
		lines = append(lines, fmt.Sprintf("  %-20s%s → %s", "Disk autoscaling:", boolLabel(diff.DiskAutoscalingEnabled.Before), boolLabel(diff.DiskAutoscalingEnabled.After)))
	}
	if diff.HighAvailabilityEnabled != nil {
		lines = append(lines, fmt.Sprintf("  %-20s%s → %s", "High availability:", boolLabel(diff.HighAvailabilityEnabled.Before), boolLabel(diff.HighAvailabilityEnabled.After)))
	}
	if diff.IPAllowList != nil {
		lines = append(lines, fmt.Sprintf("  %-20s%s → %s", "IP allow-list:", ipAllowListLabel(diff.IPAllowList.Before), ipAllowListLabel(diff.IPAllowList.After)))
	}
	return strings.Join(lines, "\n")
}

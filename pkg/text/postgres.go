package text

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/jedib0t/go-pretty/table"

	"github.com/render-oss/cli/internal/ipallowlist"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/postgres"
)

func PostgresTable(v []*postgres.Model) string {
	t := newTable()
	t.AppendHeader(table.Row{"Name", "Project", "Environment", "Plan", "Region", "Status", "ID"})
	if len(v) == 0 {
		t.SetCaption("No Postgres databases found.")
	}
	for _, m := range v {
		t.AppendRow(table.Row{
			m.Name(),
			m.ProjectName(),
			m.EnvironmentName(),
			string(m.Postgres.Plan),
			string(m.Postgres.Region),
			string(m.Postgres.Status),
			m.ID(),
		})
	}
	return FormatString(t.Render())
}

// PostgresDetail formats a Postgres instance detail for text output.
// Does NOT include an action prefix (e.g., "Created") — callers should prepend
// their own action prefix in the formatText closure passed to command.NonInteractive.
func PostgresDetail(pg *client.PostgresDetail) string {
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
	if pg.ParameterOverrides != nil && len(*pg.ParameterOverrides) > 0 {
		keys := slices.Sorted(maps.Keys(*pg.ParameterOverrides))
		var b strings.Builder
		b.WriteString("Parameter overrides:")
		for _, k := range keys {
			fmt.Fprintf(&b, "\n  %s: %s", k, (*pg.ParameterOverrides)[k])
		}
		lines = append(lines, b.String())
	}
	return strings.Join(lines, "\n")
}

func PostgresGetDetail(pg *client.PostgresDetail, conn *client.PostgresConnectionInfo) string {
	detail := PostgresDetail(pg)
	if conn == nil {
		return detail
	}
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

// PostgresUpdateDiff renders the user-visible changes between before and after
// snapshots of a Postgres instance, showing only the fields that actually
// changed. Returns an empty string when nothing changed (the cmd layer can
// then surface a "no changes" message).
//
// Label column is padded to 20 characters so the arrows align:
//
//	"  High availability: disabled → enabled"
func PostgresUpdateDiff(before, after *client.PostgresDetail) string {
	var lines []string

	if before.Name != after.Name {
		lines = append(lines, fmt.Sprintf("  %-20s%s → %s", "Name:", before.Name, after.Name))
	}
	if before.Plan != after.Plan {
		lines = append(lines, fmt.Sprintf("  %-20s%s → %s", "Plan:", string(before.Plan), string(after.Plan)))
	}
	if diskSizeLabel(before.DiskSizeGB) != diskSizeLabel(after.DiskSizeGB) {
		lines = append(lines, fmt.Sprintf("  %-20s%s → %s", "Disk size:", diskSizeLabel(before.DiskSizeGB), diskSizeLabel(after.DiskSizeGB)))
	}
	if before.DiskAutoscalingEnabled != after.DiskAutoscalingEnabled {
		lines = append(lines, fmt.Sprintf("  %-20s%s → %s", "Disk autoscaling:", boolLabel(before.DiskAutoscalingEnabled), boolLabel(after.DiskAutoscalingEnabled)))
	}
	if before.HighAvailabilityEnabled != after.HighAvailabilityEnabled {
		lines = append(lines, fmt.Sprintf("  %-20s%s → %s", "High availability:", boolLabel(before.HighAvailabilityEnabled), boolLabel(after.HighAvailabilityEnabled)))
	}
	if !ipallowlist.Equal(before.IpAllowList, after.IpAllowList) {
		lines = append(lines, fmt.Sprintf("  %-20s%s → %s", "IP allow-list:", ipAllowListLabel(before.IpAllowList), ipAllowListLabel(after.IpAllowList)))
	}
	// ParameterOverrides is a map; rather than a noisy per-key diff we flag that
	// it changed. The full new state is shown in the PostgresDetail block below.
	if !reflect.DeepEqual(before.ParameterOverrides, after.ParameterOverrides) {
		lines = append(lines, fmt.Sprintf("  %-20s %s", "Parameter overrides:", "updated"))
	}

	return strings.Join(lines, "\n")
}

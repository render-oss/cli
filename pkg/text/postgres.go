package text

import (
	"fmt"
	"strings"

	"github.com/render-oss/cli/pkg/client"
)

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
	return strings.Join(lines, "\n")
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

package text

import (
	"fmt"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/table"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/utils"
)

func SandboxSnapshotTable(snapshots []*sandboxesclient.SandboxSnapshot) string {
	t := newTable()
	t.AppendHeader(table.Row{"ID", "Kind", "Status", "Plan", "Size", "Expires", "Captured"})
	for _, s := range snapshots {
		t.AppendRow(table.Row{
			s.Id,
			s.Kind,
			s.Status,
			s.Plan,
			snapshotSize(s.SizeBytes),
			snapshotTime(s.ExpiresAt),
			snapshotTime(s.CapturedAt),
		})
	}
	return FormatString(t.Render())
}

func SandboxSnapshotDetail(s *sandboxesclient.SandboxSnapshot) string {
	lines := []string{
		fmt.Sprintf("ID:             %s", s.Id),
		fmt.Sprintf("Kind:           %s", s.Kind),
		fmt.Sprintf("Status:         %s", s.Status),
		fmt.Sprintf("Plan:           %s", s.Plan),
		fmt.Sprintf("Size:           %s", snapshotSize(s.SizeBytes)),
		fmt.Sprintf("Sandbox group:  %s", s.SandboxGroupId),
		fmt.Sprintf("Source sandbox: %s", s.SourceSandboxId),
		fmt.Sprintf("Requested:      %s", s.RequestedAt.UTC().Format(time.RFC3339)),
		fmt.Sprintf("Captured:       %s", snapshotTime(s.CapturedAt)),
		fmt.Sprintf("Expires:        %s", snapshotTime(s.ExpiresAt)),
	}
	if s.Error != nil && *s.Error != "" {
		lines = append(lines, fmt.Sprintf("Error:          %s", *s.Error))
	}
	return FormatString(strings.Join(lines, "\n"))
}

func snapshotSize(bytes *int64) string {
	if bytes == nil {
		return "-"
	}
	return utils.FormatBytes(*bytes)
}

func snapshotTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.UTC().Format(time.RFC3339)
}

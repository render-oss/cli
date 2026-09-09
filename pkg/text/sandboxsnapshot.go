package text

import (
	"fmt"
	"strings"
	"time"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/utils"
)

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

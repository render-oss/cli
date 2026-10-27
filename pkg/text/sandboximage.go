package text

import (
	"fmt"
	"strings"
	"time"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

func SandboxImageDetail(b *sandboxesclient.SandboxBuild) string {
	lines := []string{
		fmt.Sprintf("ID:             %s", b.Id),
		fmt.Sprintf("Name:           %s", b.Name),
		fmt.Sprintf("Status:         %s", b.Status),
		fmt.Sprintf("Source:         %s", imageSourceString(b)),
		fmt.Sprintf("Source digest:  %s", b.SourceDigest),
		fmt.Sprintf("Sandbox group:  %s", b.SandboxGroupId),
		fmt.Sprintf("Created:        %s", b.CreatedAt.UTC().Format(time.RFC3339)),
		fmt.Sprintf("Finished:       %s", optionalTime(b.FinishedAt)),
	}
	if b.FailureReason != nil && *b.FailureReason != "" {
		lines = append(lines, fmt.Sprintf("Failure reason: %s", *b.FailureReason))
	}
	return FormatString(strings.Join(lines, "\n"))
}

func imageSourceString(b *sandboxesclient.SandboxBuild) string {
	separator := ":"
	// A tag never contains a colon, so one marks a digest.
	if strings.Contains(b.Source.Reference, ":") {
		separator = "@"
	}
	return b.Source.Repository + "/" + b.Source.Image + separator + b.Source.Reference
}

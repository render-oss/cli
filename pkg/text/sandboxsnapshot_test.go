package text_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/text"
)

func TestSandboxSnapshotDetail(t *testing.T) {
	size := int64(3 * 1024 * 1024)
	captured := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	snapshot := &sandboxesclient.SandboxSnapshot{
		Id:              "snp-abc",
		SandboxGroupId:  "sbg-abc",
		SourceSandboxId: "sbx-abc",
		Kind:            sandboxesclient.Filesystem,
		Status:          sandboxesclient.SandboxSnapshotStatusAvailable,
		Plan:            sandboxesclient.Starter,
		RequestedAt:     captured.Add(-time.Minute),
		CapturedAt:      &captured,
		SizeBytes:       &size,
	}

	out := text.SandboxSnapshotDetail(snapshot)

	for _, want := range []string{"snp-abc", "sbg-abc", "sbx-abc", "filesystem", "available", "starter", "3.0 MB", "2026-09-01T10:00:00Z"} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "Error:")
}

func TestSandboxSnapshotDetail_FailedShowsError(t *testing.T) {
	msg := "disk full"
	snapshot := &sandboxesclient.SandboxSnapshot{
		Id:          "snp-bad",
		Kind:        sandboxesclient.Filesystem,
		Status:      sandboxesclient.SandboxSnapshotStatusFailed,
		Plan:        sandboxesclient.Starter,
		RequestedAt: time.Now(),
		Error:       &msg,
	}

	out := text.SandboxSnapshotDetail(snapshot)

	assert.Contains(t, out, "failed")
	assert.Contains(t, out, "disk full")
}

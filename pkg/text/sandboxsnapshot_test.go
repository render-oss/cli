package text_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/text"
)

func TestSandboxSnapshotTable_ContainsHeadersAndRow(t *testing.T) {
	captured := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	expires := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	size := int64(1536)
	snapshots := []*sandboxesclient.SandboxSnapshot{
		{
			Id:              "snp-abc",
			SandboxGroupId:  "sbg-abc",
			SourceSandboxId: "sbx-abc",
			Kind:            sandboxesclient.Runtime,
			Status:          sandboxesclient.SandboxSnapshotStatusAvailable,
			Plan:            sandboxesclient.Standard,
			RequestedAt:     captured.Add(-time.Minute),
			CapturedAt:      &captured,
			ExpiresAt:       expires,
			SizeBytes:       &size,
		},
	}

	out := text.SandboxSnapshotTable(snapshots)

	for _, want := range []string{
		"ID", "KIND", "STATUS", "PLAN", "SIZE", "EXPIRES", "CAPTURED",
		"snp-abc", "runtime", "available", "standard", "1.5 KB",
		"2026-09-08T10:00:00Z", "2026-09-01T10:00:00Z",
	} {
		assert.Contains(t, out, want)
	}
}

func TestSandboxSnapshotTable_NullFieldsShowDash(t *testing.T) {
	snapshots := []*sandboxesclient.SandboxSnapshot{
		{
			Id:          "snp-new",
			Kind:        sandboxesclient.Filesystem,
			Status:      sandboxesclient.SandboxSnapshotStatusCreating,
			Plan:        sandboxesclient.Starter,
			RequestedAt: time.Now(),
			ExpiresAt:   time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC),
		},
	}

	out := text.SandboxSnapshotTable(snapshots)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 2)
	fields := strings.Fields(lines[1])
	assert.Equal(t, []string{"snp-new", "filesystem", "creating", "starter", "-", "2026-09-08T10:00:00Z", "-"}, fields)
}

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

package text_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/text"
)

func newBuild(status sandboxesclient.SandboxBuildStatus) *sandboxesclient.SandboxBuild {
	b := &sandboxesclient.SandboxBuild{
		Id:             "sbb-abc",
		SandboxGroupId: "sbg-abc",
		Name:           "python",
		SourceDigest:   "sha256:c19d4a1",
		Status:         status,
		CreatedAt:      time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC),
	}
	b.Source.Repository, b.Source.Image, b.Source.Reference = "docker.io/library", "python", "3.13"
	return b
}

func TestSandboxBuildDetail_SucceededBuild(t *testing.T) {
	b := newBuild(sandboxesclient.SandboxBuildStatusSucceeded)
	finished := b.CreatedAt.Add(10 * time.Second)
	b.FinishedAt = &finished

	out := text.SandboxBuildDetail(b)

	for _, want := range []string{
		"ID:             sbb-abc",
		"Name:           python",
		"Status:         succeeded",
		"Source:         docker.io/library/python:3.13",
		"Source digest:  sha256:c19d4a1",
		"Sandbox group:  sbg-abc",
		"Created:        2026-10-01T10:00:00Z",
		"Finished:       2026-10-01T10:00:10Z",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "Failure reason")
}

func TestSandboxBuildDetail_RunningBuildHasNoFinishTime(t *testing.T) {
	out := text.SandboxBuildDetail(newBuild(sandboxesclient.SandboxBuildStatusRunning))

	assert.Contains(t, out, "Status:         running")
	assert.Contains(t, out, "Finished:       -")
}

func TestSandboxBuildDetail_FailedBuildShowsReason(t *testing.T) {
	b := newBuild(sandboxesclient.SandboxBuildStatusFailed)
	reason := "image is not linux/amd64"
	b.FailureReason = &reason

	out := text.SandboxBuildDetail(b)

	assert.Contains(t, out, "Status:         failed")
	assert.Contains(t, out, "Failure reason: image is not linux/amd64")
}

func TestSandboxBuildDetail_DigestReference(t *testing.T) {
	b := newBuild(sandboxesclient.SandboxBuildStatusSucceeded)
	b.Source.Reference = "sha256:c19d4a1"

	assert.Contains(t, text.SandboxBuildDetail(b), "Source:         docker.io/library/python@sha256:c19d4a1")
}

func TestSandboxBuildDetail_KeepsPercentSigns(t *testing.T) {
	b := newBuild(sandboxesclient.SandboxBuildStatusFailed)
	b.Name = "100%"
	reason := "disk 95% full"
	b.FailureReason = &reason

	out := text.SandboxBuildDetail(b)

	assert.Contains(t, out, "Name:           100%")
	assert.Contains(t, out, "Failure reason: disk 95% full")
}

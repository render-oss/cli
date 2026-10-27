package text_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/text"
)

func newImage(status sandboxesclient.SandboxBuildStatus) *sandboxesclient.SandboxBuild {
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

func TestSandboxImageDetail_SucceededImage(t *testing.T) {
	b := newImage(sandboxesclient.SandboxBuildStatusSucceeded)
	finished := b.CreatedAt.Add(10 * time.Second)
	b.FinishedAt = &finished

	out := text.SandboxImageDetail(b)

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

func TestSandboxImageDetail_RunningImageHasNoFinishTime(t *testing.T) {
	out := text.SandboxImageDetail(newImage(sandboxesclient.SandboxBuildStatusRunning))

	assert.Contains(t, out, "Status:         running")
	assert.Contains(t, out, "Finished:       -")
}

func TestSandboxImageDetail_FailedImageShowsReason(t *testing.T) {
	b := newImage(sandboxesclient.SandboxBuildStatusFailed)
	reason := "image is not linux/amd64"
	b.FailureReason = &reason

	out := text.SandboxImageDetail(b)

	assert.Contains(t, out, "Status:         failed")
	assert.Contains(t, out, "Failure reason: image is not linux/amd64")
}

func TestSandboxImageDetail_DigestReference(t *testing.T) {
	b := newImage(sandboxesclient.SandboxBuildStatusSucceeded)
	b.Source.Reference = "sha256:c19d4a1"

	assert.Contains(t, text.SandboxImageDetail(b), "Source:         docker.io/library/python@sha256:c19d4a1")
}

func TestSandboxImageDetail_KeepsPercentSigns(t *testing.T) {
	b := newImage(sandboxesclient.SandboxBuildStatusFailed)
	b.Name = "100%"
	reason := "disk 95% full"
	b.FailureReason = &reason

	out := text.SandboxImageDetail(b)

	assert.Contains(t, out, "Name:           100%")
	assert.Contains(t, out, "Failure reason: disk 95% full")
}

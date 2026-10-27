package sandboximage

import (
	"context"
	"errors"
	"fmt"
	"time"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

// WaitTimeout bounds WaitUntilReady, including in-flight requests.
const WaitTimeout = 10 * time.Minute

// These match the schedule in pkg/sandbox.
const (
	waitInterval    = 250 * time.Millisecond
	waitMaxInterval = 2 * time.Second
)

var errWaitTimeout = errors.New("wait timed out")

// WaitUntilReady polls the image until it has succeeded, for up to
// WaitTimeout. It fails early if the import fails. It returns the last image
// it saw, even with an error.
func (s *Service) WaitUntilReady(ctx context.Context, sandboxGroupID, imageID string) (*sandboxesclient.SandboxBuild, error) {
	ctx, cancel := context.WithTimeoutCause(ctx, WaitTimeout, errWaitTimeout)
	defer cancel()

	var last *sandboxesclient.SandboxBuild
	for delay := waitInterval; ; delay = min(2*delay, waitMaxInterval) {
		image, err := s.repo.Get(ctx, sandboxGroupID, imageID)
		if err != nil {
			return last, waitError(ctx, imageID, last, err)
		}
		last = image

		switch image.Status {
		case sandboxesclient.SandboxBuildStatusSucceeded:
			return image, nil
		case sandboxesclient.SandboxBuildStatusFailed:
			if image.FailureReason != nil && *image.FailureReason != "" {
				return image, fmt.Errorf("image %s failed to import: %s", imageID, *image.FailureReason)
			}
			return image, fmt.Errorf("image %s failed to import", imageID)
		}

		select {
		case <-ctx.Done():
			return image, waitError(ctx, imageID, image, ctx.Err())
		case <-time.After(delay):
		}
	}
}

// waitError names the timeout and last status when the wait's own deadline
// stopped it, and wraps a cancellation from the caller.
func waitError(ctx context.Context, imageID string, last *sandboxesclient.SandboxBuild, err error) error {
	switch {
	case errors.Is(context.Cause(ctx), errWaitTimeout):
		if last == nil {
			return fmt.Errorf("image %s was not ready within %dm", imageID, int(WaitTimeout.Minutes()))
		}
		return fmt.Errorf("image %s was not ready within %dm (last status: %s)", imageID, int(WaitTimeout.Minutes()), last.Status)
	case ctx.Err() != nil:
		return fmt.Errorf("waiting for image %s to be ready: %w", imageID, ctx.Err())
	default:
		return err
	}
}

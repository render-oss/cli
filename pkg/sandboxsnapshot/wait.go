package sandboxsnapshot

import (
	"context"
	"errors"
	"fmt"
	"time"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

// WaitTimeout bounds WaitUntilAvailable, including in-flight requests.
const WaitTimeout = 5 * time.Minute

// These match the schedule in pkg/sandbox.
const (
	waitInterval    = 250 * time.Millisecond
	waitMaxInterval = 2 * time.Second
)

var errWaitTimeout = errors.New("wait timed out")

// WaitUntilAvailable polls the snapshot until it is available, for up to
// WaitTimeout. It fails early if the capture fails. It returns the last
// snapshot it saw, even with an error.
func (s *Service) WaitUntilAvailable(ctx context.Context, sandboxGroupID, snapshotID string) (*sandboxesclient.SandboxSnapshot, error) {
	ctx, cancel := context.WithTimeoutCause(ctx, WaitTimeout, errWaitTimeout)
	defer cancel()

	var last *sandboxesclient.SandboxSnapshot
	for delay := waitInterval; ; delay = min(2*delay, waitMaxInterval) {
		snapshot, err := s.repo.Get(ctx, sandboxGroupID, snapshotID)
		if err != nil {
			return last, waitError(ctx, snapshotID, last, err)
		}
		last = snapshot

		switch snapshot.Status {
		case sandboxesclient.SandboxSnapshotStatusAvailable:
			return snapshot, nil
		case sandboxesclient.SandboxSnapshotStatusFailed:
			if snapshot.Error != nil && *snapshot.Error != "" {
				return snapshot, fmt.Errorf("snapshot %s failed to capture: %s", snapshotID, *snapshot.Error)
			}
			return snapshot, fmt.Errorf("snapshot %s failed to capture", snapshotID)
		}

		select {
		case <-ctx.Done():
			return snapshot, waitError(ctx, snapshotID, snapshot, ctx.Err())
		case <-time.After(delay):
		}
	}
}

// waitError names the timeout and last status when the wait's own deadline
// stopped it, and wraps a cancellation from the caller.
func waitError(ctx context.Context, snapshotID string, last *sandboxesclient.SandboxSnapshot, err error) error {
	switch {
	case errors.Is(context.Cause(ctx), errWaitTimeout):
		if last == nil {
			return fmt.Errorf("snapshot %s was not available within %dm", snapshotID, int(WaitTimeout.Minutes()))
		}
		return fmt.Errorf("snapshot %s was not available within %dm (last status: %s)", snapshotID, int(WaitTimeout.Minutes()), last.Status)
	case ctx.Err() != nil:
		return fmt.Errorf("waiting for snapshot %s to be available: %w", snapshotID, ctx.Err())
	default:
		return err
	}
}

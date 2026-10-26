package sandbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

// WaitTimeout bounds WaitUntilRunning, including in-flight requests.
const WaitTimeout = 60 * time.Second

// The delay between checks starts short for a fast warm start, then doubles
// to stay under the API rate limit. The SDK uses the same schedule.
const (
	waitInterval    = 250 * time.Millisecond
	waitMaxInterval = 2 * time.Second
)

var errWaitTimeout = errors.New("wait timed out")

// WaitUntilRunning polls the sandbox until it is running, for up to
// WaitTimeout. It fails early if the sandbox can't run. It returns the last
// sandbox it saw, even with an error. onEvent, if set, gets each new status.
func (s *Service) WaitUntilRunning(ctx context.Context, id string, onEvent func(*sandboxclient.Sandbox)) (*sandboxclient.Sandbox, error) {
	ctx, cancel := context.WithTimeoutCause(ctx, WaitTimeout, errWaitTimeout)
	defer cancel()

	var last *sandboxclient.Sandbox
	for delay := waitInterval; ; delay = min(2*delay, waitMaxInterval) {
		sb, err := s.repo.GetSandbox(ctx, id)
		if err != nil {
			return last, waitError(ctx, id, last, err)
		}
		if onEvent != nil && (last == nil || sb.Status != last.Status) {
			onEvent(sb)
		}
		last = sb

		switch sb.Status {
		case sandboxclient.SandboxStatusRunning:
			return sb, nil
		case sandboxclient.SandboxStatusCreating, sandboxclient.SandboxStatusResuming:
		default:
			return sb, fmt.Errorf("sandbox %s is %s, not running", id, sb.Status)
		}

		select {
		case <-ctx.Done():
			return sb, waitError(ctx, id, sb, ctx.Err())
		case <-time.After(delay):
		}
	}
}

// waitError names the timeout and last status when the wait's own deadline
// stopped it, and wraps a cancellation from the caller.
func waitError(ctx context.Context, id string, last *sandboxclient.Sandbox, err error) error {
	switch {
	case errors.Is(context.Cause(ctx), errWaitTimeout):
		if last == nil {
			return fmt.Errorf("sandbox %s did not run within %ds", id, int(WaitTimeout.Seconds()))
		}
		return fmt.Errorf("sandbox %s did not run within %ds (last status: %s)", id, int(WaitTimeout.Seconds()), last.Status)
	case ctx.Err() != nil:
		return fmt.Errorf("waiting for sandbox %s to run: %w", id, ctx.Err())
	default:
		return err
	}
}

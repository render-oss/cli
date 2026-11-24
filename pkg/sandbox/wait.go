package sandbox

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

// WaitTimeout bounds WaitUntilRunning and WaitUntilSuspended, including in-flight requests.
const WaitTimeout = 60 * time.Second

// The delay between checks starts short for a fast warm start, then doubles
// to stay under the API rate limit. The SDK uses the same schedule.
const (
	waitInterval    = 250 * time.Millisecond
	waitMaxInterval = 2 * time.Second
)

var errWaitTimeout = errors.New("wait timed out")

// statusWait describes one target status and the statuses that can still lead
// to it. verb completes "did not ... within" and adj completes "not ...".
type statusWait struct {
	target  sandboxclient.SandboxStatus
	pending []sandboxclient.SandboxStatus
	verb    string
	adj     string
}

var (
	waitRunning = statusWait{
		target:  sandboxclient.SandboxStatusRunning,
		pending: []sandboxclient.SandboxStatus{sandboxclient.SandboxStatusCreating, sandboxclient.SandboxStatusResuming},
		verb:    "run",
		adj:     "running",
	}
	waitSuspended = statusWait{
		target:  sandboxclient.SandboxStatusSuspended,
		pending: []sandboxclient.SandboxStatus{sandboxclient.SandboxStatusSuspending},
		verb:    "suspend",
		adj:     "suspended",
	}
)

// WaitUntilRunning polls the sandbox until it is running, for up to
// WaitTimeout. It fails early if the sandbox can't run. It returns the last
// sandbox it saw, even with an error. onEvent, if set, gets each new status.
func (s *Service) WaitUntilRunning(ctx context.Context, id string, onEvent func(*sandboxclient.Sandbox)) (*sandboxclient.Sandbox, error) {
	return s.waitFor(ctx, id, waitRunning, onEvent)
}

// WaitUntilSuspended polls the sandbox until it is suspended, for up to
// WaitTimeout. It fails early if the sandbox leaves suspending for any other
// status. It returns the last sandbox it saw, even with an error. onEvent, if
// set, gets each new status.
func (s *Service) WaitUntilSuspended(ctx context.Context, id string, onEvent func(*sandboxclient.Sandbox)) (*sandboxclient.Sandbox, error) {
	return s.waitFor(ctx, id, waitSuspended, onEvent)
}

func (s *Service) waitFor(ctx context.Context, id string, w statusWait, onEvent func(*sandboxclient.Sandbox)) (*sandboxclient.Sandbox, error) {
	ctx, cancel := context.WithTimeoutCause(ctx, WaitTimeout, errWaitTimeout)
	defer cancel()

	var last *sandboxclient.Sandbox
	for delay := waitInterval; ; delay = min(2*delay, waitMaxInterval) {
		sb, err := s.repo.GetSandbox(ctx, id)
		if err != nil {
			return last, waitError(ctx, id, w, last, err)
		}
		if onEvent != nil && (last == nil || sb.Status != last.Status) {
			onEvent(sb)
		}
		last = sb

		if sb.Status == w.target {
			return sb, nil
		}
		if !slices.Contains(w.pending, sb.Status) {
			return sb, fmt.Errorf("sandbox %s is %s, not %s", id, sb.Status, w.adj)
		}

		select {
		case <-ctx.Done():
			return sb, waitError(ctx, id, w, sb, ctx.Err())
		case <-time.After(delay):
		}
	}
}

// waitError names the timeout and last status when the wait's own deadline
// stopped it, and wraps a cancellation from the caller.
func waitError(ctx context.Context, id string, w statusWait, last *sandboxclient.Sandbox, err error) error {
	switch {
	case errors.Is(context.Cause(ctx), errWaitTimeout):
		if last == nil {
			return fmt.Errorf("sandbox %s did not %s within %ds", id, w.verb, int(WaitTimeout.Seconds()))
		}
		return fmt.Errorf("sandbox %s did not %s within %ds (last status: %s)", id, w.verb, int(WaitTimeout.Seconds()), last.Status)
	case ctx.Err() != nil:
		return fmt.Errorf("waiting for sandbox %s to %s: %w", id, w.verb, ctx.Err())
	default:
		return err
	}
}

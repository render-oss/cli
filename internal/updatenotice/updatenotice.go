// Package updatenotice tells people at a terminal, and AI agents, when a newer
// CLI release is available, at most once a day. The release lookup runs in the background
// while the command runs and is cached on disk, so most invocations never touch
// the network.
package updatenotice

import (
	"context"
	"fmt"
	"time"

	"github.com/blang/semver/v4"

	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/command"
)

const (
	// checkInterval is how long a lookup result, success or failure, is reused
	// before GitHub is asked again. Releases ship about once a week.
	checkInterval = 4 * time.Hour
	// notifyInterval is the least time between two notices.
	notifyInterval = 24 * time.Hour
	// exitGracePeriod is how long Show waits for a lookup that is still running
	// when the command finishes. After that the lookup is cancelled, nothing is
	// recorded, and the next command tries again: a quick command should not
	// wait on GitHub.
	exitGracePeriod = 300 * time.Millisecond
	// homebrewGracePeriod holds the notice back for Homebrew installs until the
	// homebrew-core formula has had time to pick up a new release; before that,
	// `brew upgrade render` finds nothing to upgrade. The formula and its bottle
	// have landed within 1 to 2.5 hours of recent releases.
	homebrewGracePeriod = 6 * time.Hour
)

// Conditions are the runtime facts that decide whether the notice may run.
type Conditions struct {
	// Interactive reports whether a person is at the terminal: every stream is
	// a TTY, the terminal is not dumb, and the CLI is not running in CI.
	Interactive bool
	// Agent reports whether an AI coding agent is running the CLI. Agents get
	// the notice as one plain line, whether or not they run in a terminal.
	Agent bool
}

// GitHubReleaseClient looks up the latest published CLI release on GitHub.
type GitHubReleaseClient interface {
	LatestRelease(ctx context.Context) (Release, error)
}

// Check is an update check started by [Start]. A nil *Check is valid and never
// shows anything.
type Check struct {
	current semver.Version
	install Install
	line    bool
	cancel  context.CancelFunc
	done    chan struct{}
	// state is the lookup result. It is written before done is closed and
	// must only be read after.
	state state
}

// Start begins an update check for currentVersion, returning nil when no notice
// can be shown on this run. Runs with neither a person at the terminal nor an
// agent, opted out (RENDER_CLI_DISABLE_UPDATE_NOTIFIER), built from source, or
// already notified within the last day never make a network request.
func Start(conditions Conditions, client GitHubReleaseClient, currentVersion string) *Check {
	if !(conditions.Interactive || conditions.Agent) || cfg.UpdateNotifierDisabled() {
		return nil
	}

	current, err := semver.Parse(currentVersion)
	if err != nil {
		// Builds from source report "dev" and have nothing to compare against.
		return nil
	}

	var cached state
	if s, err := readState(); err == nil {
		cached = *s
	}
	if time.Since(cached.NotifiedAt) < notifyInterval {
		return nil
	}

	c := &Check{
		current: current,
		install: DetectInstall(),
		line:    conditions.Agent,
		done:    make(chan struct{}),
	}
	if time.Since(cached.CheckedAt) < checkInterval {
		c.state = cached
		c.cancel = func() {}
		close(c.done)
		return c
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	go func() {
		defer close(c.done)
		c.state = refresh(ctx, client, cached)
	}()
	return c
}

// refresh asks client for the latest release and records the attempt. A failed
// lookup is recorded too, keeping the previous result, so an offline machine
// does not retry on every command. A lookup cancelled by Show is not recorded.
// The attempt is merged into whatever is saved by then, so a notice another
// command showed during the lookup stays recorded.
func refresh(ctx context.Context, client GitHubReleaseClient, previous state) state {
	latest, err := client.LatestRelease(ctx)
	if ctx.Err() != nil {
		return previous
	}

	return updateState(previous, func(s *state) {
		s.CheckedAt = time.Now()
		if err == nil {
			s.LatestVersion = latest.Version.String()
			s.PublishedAt = latest.PublishedAt
		}
	})
}

// Cancel stops a lookup still in flight and waits for it to return, for runs
// that will not show the notice. Nothing is recorded, so the next run checks
// again.
func (c *Check) Cancel() {
	if c == nil {
		return
	}
	c.cancel()
	<-c.done
}

// Show prints the notice to s when a newer release exists, then records that
// it was shown so the next notice waits a day. It waits at most
// exitGracePeriod for a lookup still in flight, and reports whether the notice
// was printed.
func (c *Check) Show(s *command.Stream) bool {
	if c == nil {
		return false
	}

	select {
	case <-c.done:
	case <-time.After(exitGracePeriod):
		c.cancel()
		<-c.done
		return false
	}
	c.cancel()

	latest := c.state.latest()
	if latest == nil || !latest.GT(c.current) {
		return false
	}
	if c.install == InstallHomebrew && time.Since(c.state.PublishedAt) < homebrewGracePeriod {
		return false
	}

	// Another command may have shown the notice since Start read the state.
	if saved, err := readState(); err == nil && time.Since(saved.NotifiedAt) < notifyInterval {
		return false
	}

	var err error
	if c.line {
		_, err = fmt.Fprintln(s, RenderLine(c.current.String(), latest.String(), c.install))
	} else {
		_, err = fmt.Fprintf(s, "\n%s\n", Render(s, c.current.String(), latest.String(), c.install))
	}
	if err != nil {
		return false
	}

	updateState(c.state, func(s *state) { s.NotifiedAt = time.Now() })
	return true
}

// Package analyticsnotice implements the CLI's telemetry disclosure: a notice
// explaining what the CLI collects and how to opt out, shown once per machine
// before analytics can send anything in non-CI runs. CI automation bypasses the
// notice gate. The package decides whether to show the notice, builds its copy,
// and persists a marker in the CLI state directory so it isn't repeated.
//
// Callers must invoke [ShowIfNeeded] before a TUI takes over the terminal, so
// the notice remains visible after it is recorded as shown.
package analyticsnotice

import (
	"fmt"

	"github.com/render-oss/cli/pkg/analytics"
	"github.com/render-oss/cli/pkg/command"
)

// Conditions contains the runtime signals [ShowIfNeeded] evaluates.
type Conditions struct {
	// CI reports whether the CLI is running in a continuous integration
	// environment. CI runs bypass the notice gate.
	CI bool
	// StderrTTY reports whether stderr is connected to a terminal.
	StderrTTY bool
}

// ShowIfNeeded decides whether to show the analytics notice, writes it to s,
// and records it as shown. It returns whether the caller should skip analytics
// for this run. Outside CI, analytics can proceed only when an existing marker
// proves the notice was shown on an earlier run. CI is an automation exception:
// it shows nothing, writes no marker, and does not ask the caller to skip
// analytics. Explicit analytics opt-outs remain the caller's responsibility.
//
// optOutReason is empty when telemetry is on, or names the opt-out mechanism
// in effect (see [buildNotice]).
//
// The marker is written after the notice. With no file lock, two concurrent
// first runs may both print the notice.
//
// ShowIfNeeded never returns an error. It shows nothing if it cannot determine
// whether the marker exists, and may show the notice again after a failed
// marker write. Non-TTY runs and marker read failures keep analytics suppressed
// until a later run can show the notice and persist the marker. A telemetry
// setup failure must not fail a command.
func ShowIfNeeded(s *command.Stream, conditions Conditions, optOutReason analytics.OptOutReason) (skipAnalytics bool) {
	if conditions.CI {
		return false
	}

	exists, err := markerExists()
	if err != nil {
		return true
	}
	if exists {
		return false
	}

	if !conditions.StderrTTY {
		return true
	}

	// The trailing blank line separates the notice from whatever the caller
	// prints next, and belongs here rather than at each call site: only this
	// function knows whether the notice was actually shown.
	if _, err := fmt.Fprintf(s, "%s\n\n", buildNotice(s, optOutReason)); err != nil {
		return true
	}
	_ = writeMarker()
	return true
}

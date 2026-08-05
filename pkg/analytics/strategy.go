package analytics

import "fmt"

const (
	envValueAuto = "auto"
	envValueSync = "sync"
)

type sendStrategy int

const (
	// Send in a child background process
	strategyDetached sendStrategy = iota
	// Send as part of the current command's process
	strategySync
)

type strategyInputs struct {
	isCI               bool
	loggingEnabled     bool
	configuredStrategy string
}

// resolveStrategy returns the send strategy selected by the highest-priority
// matching input and an optional diagnostic for an unknown configured value.
// It answers how to send, so callers only reach it once they have decided to
// send at all.
//
// The diagnostic is independent of which rule selects the strategy — an
// unknown value deserves a report even when a higher-priority rule would
// have overridden it anyway. The caller decides where to write a non-empty
// diagnostic.
func resolveStrategy(inputs strategyInputs) (sendStrategy, string) {
	var diagnostic string
	if !isKnownStrategyValue(inputs.configuredStrategy) {
		diagnostic = fmt.Sprintf(
			"unknown RENDER_CLI_ANALYTICS_STRATEGY value %q; ignoring it",
			inputs.configuredStrategy,
		)
	}

	if inputs.isCI {
		return strategySync, diagnostic
	}

	// If we are sending with logs on, we need to send sync
	// in order for the user to see the logs
	if inputs.loggingEnabled {
		return strategySync, diagnostic
	}

	if inputs.configuredStrategy == envValueSync {
		return strategySync, diagnostic
	}
	return strategyDetached, diagnostic
}

// isKnownStrategyValue reports whether value is a RENDER_CLI_ANALYTICS_STRATEGY
// setting the CLI understands. Empty means unset, which is valid.
func isKnownStrategyValue(value string) bool {
	return value == "" || value == envValueAuto || value == envValueSync
}

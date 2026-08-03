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
	ci                 string
	loggingEnabled     bool
	sendingEnabled     bool
	configuredStrategy string
}

// resolveStrategy returns the send strategy selected by the highest-priority
// matching input and an optional diagnostic for an unknown configured value.
// The caller decides where to write a non-empty diagnostic.
func resolveStrategy(inputs strategyInputs) (sendStrategy, string) {
	if inputs.ci == "true" {
		return strategySync, ""
	}

	// If we are sending with logs on, we need to send sync
	// in order for the user to see the logs
	if inputs.loggingEnabled && inputs.sendingEnabled {
		return strategySync, ""
	}

	switch inputs.configuredStrategy {
	case envValueSync:
		return strategySync, ""
	case "", envValueAuto:
		return strategyDetached, ""
	default:
		if inputs.loggingEnabled {
			return strategyDetached, fmt.Sprintf(
				"unknown RENDER_CLI_ANALYTICS_STRATEGY value %q; using auto",
				inputs.configuredStrategy,
			)
		}
		return strategyDetached, ""
	}
}

package analytics

import "github.com/render-oss/cli/pkg/command"

// envSetter is the part of *testing.T that [ClearSignalEnvVars] needs
type envSetter interface {
	Setenv(key, value string)
}

// ClearSignalEnvVars clears every environment variable read by agent, CI, and
// terminal signal detection, so an emitted event's agent_signals, ci_signals,
// and is_term_dumb fields are identical on a dev machine running under an AI
// agent, in a plain terminal, and on CI.
//
// This creates a blank slate so that tests can better control the environment for tests that involve analytics
func ClearSignalEnvVars(t envSetter) {
	for _, name := range signalEnvVars() {
		t.Setenv(name, "")
	}
}

// signalEnvVars returns the names of every environment variable read by agent,
// CI, or terminal signal detection. Callers clear each name, so order and
// duplicates are irrelevant.
func signalEnvVars() []string {
	names := []string{command.TermEnvVar}
	for _, signal := range supportedAgentSignals {
		names = append(names, signal.envVar)
	}
	for _, signal := range supportedCISignals {
		names = append(names, signal.envVar)
	}
	return names
}

package analytics

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/command"
)

// ClearSignalEnvVars is a test helper that helps ensure our tests behave the same regardless of what is running them (CI, agent, or human)
// This test exists to catch regressions where it is no longer clearing out all necessary environment variables.
func TestClearSignalEnvVarsSilencesDetection(t *testing.T) {
	var wantAgent []string
	for _, signal := range supportedAgentSignals {
		t.Setenv(signal.envVar, activatingValue(signal.envVar))
		wantAgent = append(wantAgent, signal.envVar)
	}
	var wantCI []string
	for _, signal := range supportedCISignals {
		t.Setenv(signal.envVar, activatingValue(signal.envVar))
		wantCI = append(wantCI, signal.envVar)
	}
	t.Setenv(command.TermEnvVar, "dumb")
	slices.Sort(wantAgent)
	slices.Sort(wantCI)

	// Assert on the whole set rather than "something was detected", so silence
	// after clearing means the clearing worked rather than the fixture having
	// failed to activate that name in the first place.
	require.Equal(t, wantAgent, DetectAgentSignals(),
		"fixture must activate every allowlisted agent signal")
	require.Equal(t, wantCI, DetectCISignals(),
		"fixture must activate every allowlisted CI signal")
	require.True(t, command.DetectTerminalSignals().DumbTerminal,
		"fixture must activate terminal detection")

	ClearSignalEnvVars(t)

	require.Empty(t, DetectAgentSignals())
	require.Empty(t, DetectCISignals())
	require.False(t, command.DetectTerminalSignals().DumbTerminal)
}

// exceptionalActivatingValues holds the signals whose activation rule needs a
// specific shape rather than a generic truthy marker. Everything else activates
// on "1"; a new signal that needs an entry here but lacks one fails the
// whole-set assertions above rather than silently going unexercised.
var exceptionalActivatingValues = map[string]string{
	"BUILD_TAG":       "jenkins-42",
	"OPENCODE_CLIENT": "desktop",
}

func activatingValue(envVar string) string {
	if value, ok := exceptionalActivatingValues[envVar]; ok {
		return value
	}
	return "1"
}

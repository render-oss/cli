package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoginDoesNotEmitAnalytics(t *testing.T) {
	// The harness's API key forces the "already authenticated" path. This test
	// cares about reaching command completion without emitting analytics.
	harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
		noticeMarkerPresent: true,
		allowSubprocess:     true,
	})

	result := harness.execute("login")

	require.Equal(t, 0, result.ExitCode)
	require.False(t, result.AnalyticsEligible)
	require.True(t, result.AnalyticsNoticeEligible)
	require.Contains(t, result.Stdout, "already authenticated")
	require.Empty(t, harness.server.CliTelemetry.Instances)
}

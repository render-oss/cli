package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/fakes/renderapi"
)

func TestLoginDoesNotEmitAnalytics(t *testing.T) {
	server := renderapi.NewServer(t)
	// This is an easy way to force us down the "already authenticated" path
	// For this test we just care about getting to the command completion to validate that no analytics events were sent.
	t.Setenv("RENDER_API_KEY", "test-api-key")

	result := executeWithAnalytics(t, server, t.TempDir(), true, "login")

	require.Equal(t, 0, result.Result.ExitCode)
	require.False(t, result.Result.AnalyticsEligible)
	require.True(t, result.Result.AnalyticsNoticeEligible)
	require.Contains(t, result.Stdout, "already authenticated")
	require.Empty(t, server.CliTelemetry.Instances)
}

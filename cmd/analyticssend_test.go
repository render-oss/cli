package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	telemetryclient "github.com/render-oss/cli/pkg/client/clitelemetry"
)

func TestAnalyticsSendSendsEventWithoutEmittingAnalyticsForItself(t *testing.T) {
	harness := newAnalyticsHarness(t, true)
	payload := client.CreateCliTelemetryEventJSONRequestBody{
		Command:        "render services list",
		CompletionKind: telemetryclient.Success,
		ExitCode:       0,
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	eventFilePath := writeAnalyticsEventFile(t, harness.configDir, data)

	result := harness.execute("analytics", "send", eventFilePath)

	require.Equal(t, 0, result.ExitCode)
	require.Equal(t, []client.CreateCliTelemetryEventJSONRequestBody{payload}, harness.server.CliTelemetry.Instances)
	_, err = os.Lstat(eventFilePath)
	require.ErrorIs(t, err, os.ErrNotExist, "sending the event should consume it (delete the file)")
	entries, err := os.ReadDir(filepath.Dir(eventFilePath))
	require.NoError(t, err)
	require.Empty(t, entries, "the sender command must not leave another event for itself")
}

// TestAnalyticsSendWritesDiagnosticsOnlyWhenLoggingEnabled pins the ticket's
// logging contract: send outcomes reach stderr only under RENDER_LOG_ANALYTICS.
func TestAnalyticsSendWritesDiagnosticsOnlyWhenLoggingEnabled(t *testing.T) {
	testCases := []struct {
		name            string
		logging         string
		wantDiagnostics bool
	}{
		{name: "logging disabled", logging: "", wantDiagnostics: false},
		{name: "logging enabled", logging: "1", wantDiagnostics: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// The Sender captures the logging gate when the harness constructs
			// dependencies, so the override must be in place first.
			t.Setenv("RENDER_LOG_ANALYTICS", tc.logging)
			harness := newAnalyticsHarness(t, true)
			eventFilePath := writeAnalyticsEventFile(t, harness.configDir, []byte(`{"command":"render services list"}`))

			result := harness.execute("analytics", "send", eventFilePath)

			require.Equal(t, 0, result.ExitCode)
			if tc.wantDiagnostics {
				require.Contains(t, harness.stderr.String(), "analytics response:")
			} else {
				require.Empty(t, harness.stderr.String())
			}
		})
	}
}

func TestAnalyticsSendReturnsErrorsWithoutSending(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		harness := newAnalyticsHarness(t, true)
		path := analyticsEventPath(harness.configDir)

		result := harness.execute("analytics", "send", path)

		require.Equal(t, 1, result.ExitCode)
		require.Empty(t, harness.server.CliTelemetry.Instances)
	})

	t.Run("invalid event", func(t *testing.T) {
		harness := newAnalyticsHarness(t, true)
		path := writeAnalyticsEventFile(t, harness.configDir, []byte("not json"))

		result := harness.execute("analytics", "send", path)

		require.Equal(t, 1, result.ExitCode)
		require.Empty(t, harness.server.CliTelemetry.Instances)
	})
}

// TestAnalyticsSendSilencesCobraErrorOutput covers what a failed send leaks onto
// the stderr it was handed. A caller that waits for this command shares its own
// stderr with it, so Cobra's bare "Error: ..." would appear directly after a
// command that succeeded and read as though that command had failed. Narrating
// the failure belongs to [analytics.Sender.SendFile], under the analytics prefix;
// this command only has to stay quiet while still exiting non-zero.
func TestAnalyticsSendSilencesCobraErrorOutput(t *testing.T) {
	t.Setenv("RENDER_LOG_ANALYTICS", "1")
	harness := newAnalyticsHarness(t, true)
	path := writeAnalyticsEventFile(t, harness.configDir, []byte("not json"))

	result := harness.execute("analytics", "send", path)

	require.Equal(t, 1, result.ExitCode, "a waiting caller detects a failed send through the exit code")
	require.NotContains(t, harness.stderr.String(), "Error:",
		"Cobra's error prefix would read as a failure of the user's own command")
	require.Contains(t, harness.stderr.String(), "analytics error:",
		"with logging enabled the failure is narrated under the analytics prefix instead")
}

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
	harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
		devGateOpen:         true,
		noticeMarkerPresent: true,
		allowSubprocess:     true,
	})
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
		name           string
		loggingEnabled bool
	}{
		{name: "logging disabled", loggingEnabled: false},
		{name: "logging enabled", loggingEnabled: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
				devGateOpen:         true,
				loggingEnabled:      tc.loggingEnabled,
				noticeMarkerPresent: false,
			})
			eventFilePath := writeAnalyticsEventFile(t, harness.configDir, []byte(`{"command":"render services list"}`))

			result := harness.execute("analytics", "send", eventFilePath)

			require.Equal(t, 0, result.ExitCode)
			if tc.loggingEnabled {
				require.Contains(t, result.Stderr, "analytics response:")
			} else {
				require.Empty(t, result.Stderr)
			}
		})
	}
}

func TestAnalyticsSendReturnsErrorsWithoutSending(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
			devGateOpen:         true,
			noticeMarkerPresent: false,
		})
		path := analyticsEventPath(harness.configDir)

		result := harness.execute("analytics", "send", path)

		require.Equal(t, 1, result.ExitCode)
		require.Empty(t, harness.server.CliTelemetry.Instances)
	})

	t.Run("invalid event", func(t *testing.T) {
		harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
			devGateOpen:         true,
			noticeMarkerPresent: false,
		})
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
	harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
		devGateOpen:         true,
		loggingEnabled:      true,
		noticeMarkerPresent: false,
	})
	path := writeAnalyticsEventFile(t, harness.configDir, []byte("not json"))

	result := harness.execute("analytics", "send", path)

	require.Equal(t, 1, result.ExitCode, "a waiting caller detects a failed send through the exit code")
	require.NotContains(t, result.Stderr, "Error:",
		"Cobra's error prefix would read as a failure of the user's own command")
	require.Contains(t, result.Stderr, "analytics error:",
		"with logging enabled the failure is narrated under the analytics prefix instead")
}

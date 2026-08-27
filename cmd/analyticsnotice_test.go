package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/files"
	"github.com/render-oss/cli/pkg/command"
)

// markAnalyticsNoticeShown seeds the notice marker in configDir, modelling a
// machine that has already been told what the CLI collects. Tests about
// anything other than the disclosure itself need this: without a marker the
// completion hook suppresses the analytics send for the run.
func markAnalyticsNoticeShown(t *testing.T, configDir string) {
	t.Helper()

	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
	markerPath := filepath.Join(configDir, "state", "analytics-notice-shown")
	require.NoError(t, files.Write(markerPath, nil), "seed the analytics notice marker")
}

func TestAnalyticsNoticeIsHiddenFromAnalyticsHelp(t *testing.T) {
	harness := newAnalyticsHarness(t, false)

	result := harness.execute("analytics", "--help")

	require.Equal(t, 0, result.ExitCode)
	require.NotContains(t, harness.stdout.String(), "notice")
}

func TestAnalyticsNoticeIsAnalyticsIneligible(t *testing.T) {
	harness := newAnalyticsHarness(t, true)

	result := harness.execute("analytics", "notice")

	require.False(t, result.AnalyticsEligible,
		"showing the disclosure is not user activity, and reporting it would send telemetry before the user has had any chance to opt out")
	require.Empty(t, harness.server.CliTelemetry.Instances)
}

func TestAnalyticsNoticeCommandShowsNoticeOnce(t *testing.T) {
	harness := newAnalyticsHarness(t, true)
	harness.deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{StderrTTY: true}, nil
	}
	result := harness.execute("analytics", "notice")

	require.Equal(t, 0, result.ExitCode)
	require.Empty(t, harness.stdout.String(), "the notice belongs on stderr so it never pollutes piped stdout")
	require.Contains(t, ansi.Strip(harness.stderr.String()), "The Render CLI collects usage data")

	info, err := os.Stat(harness.analyticsNoticeMarkerPath())
	require.NoError(t, err)
	require.Zero(t, info.Size())

	harness.stderr.Reset()
	result = harness.execute("analytics", "notice")

	require.Equal(t, 0, result.ExitCode)
	require.Empty(t, harness.stdout.String())
	require.Empty(t, harness.stderr.String(), "the marker must suppress a repeat notice")
}

func TestAnalyticsNoticeRejectsExtraArguments(t *testing.T) {
	harness := newAnalyticsHarness(t, false)

	result := harness.execute("analytics", "notice", "extra")

	require.NotEqual(t, 0, result.ExitCode)
}

func TestAnalyticsNoticeRequiresDevelopmentGate(t *testing.T) {
	harness := newAnalyticsHarness(t, false)
	harness.deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{StderrTTY: true}, nil
	}
	t.Setenv("RENDER_TEST_ENABLE_ANALYTICS", "")

	result := harness.execute("analytics", "notice")

	require.Equal(t, 0, result.ExitCode)
	require.Empty(t, harness.stdout.String())
	require.Empty(t, harness.stderr.String())
	require.NoFileExists(t, harness.analyticsNoticeMarkerPath())
}

package cmd

import (
	"os"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/command"
)

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

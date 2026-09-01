package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestAnalyticsNoticeIsHiddenFromAnalyticsHelp(t *testing.T) {
	for _, args := range [][]string{{"analytics", "--help"}, {"help", "analytics"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
				noticeMarkerPresent: false,
				stderrTTY:           true,
			})

			result := harness.execute(args...)

			require.Equal(t, 0, result.ExitCode)
			require.False(t, result.AnalyticsNoticeEligible)
			require.NotContains(t, result.Stdout, "notice")
			require.NoFileExists(t, harness.noticeMarkerPath())
		})
	}
}

func TestAnalyticsNoticeIsAnalyticsIneligible(t *testing.T) {
	harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
		noticeMarkerPresent: true,
		allowSubprocess:     true,
	})

	result := harness.execute("analytics", "notice")

	require.False(t, result.AnalyticsEligible,
		"showing the disclosure is not user activity, and reporting it would send telemetry before the user has had any chance to opt out")
	require.False(t, result.AnalyticsNoticeEligible)
	require.Empty(t, harness.server.CliTelemetry.Instances)
}

func TestAnalyticsNoticeCommandShowsNoticeOnce(t *testing.T) {
	harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
		noticeMarkerPresent: false,
		stderrTTY:           true,
	})
	first := harness.execute("analytics", "notice")

	require.Equal(t, 0, first.ExitCode)
	require.Empty(t, first.Stdout, "the notice belongs on stderr so it never pollutes piped stdout")
	require.Contains(t, ansi.Strip(first.Stderr), "The Render CLI collects usage data")

	info, err := os.Stat(harness.noticeMarkerPath())
	require.NoError(t, err)
	require.Zero(t, info.Size())

	second := harness.execute("analytics", "notice")

	require.Equal(t, 0, second.ExitCode)
	require.Empty(t, second.Stdout)
	require.Empty(t, second.Stderr, "the marker must suppress a repeat notice")
}

func TestAnalyticsNoticeRejectsExtraArguments(t *testing.T) {
	harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
		noticeMarkerPresent: false,
	})

	result := harness.execute("analytics", "notice", "extra")

	require.NotEqual(t, 0, result.ExitCode)
}

func TestAnalyticsNoticeIsEnabledByDefault(t *testing.T) {
	testCases := []struct {
		name string
		args []string
	}{
		{name: "regular command", args: []string{"ping"}},
		{name: "explicit notice command", args: []string{"analytics", "notice"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
				loggingEnabled:      true,
				noticeMarkerPresent: false,
				stderrTTY:           true,
				allowSubprocess:     true,
			})

			result := harness.execute(tc.args...)

			require.Equal(t, 0, result.ExitCode)
			require.Contains(t, ansi.Strip(result.Stderr), "The Render CLI collects usage data")
			require.Zero(t, result.countLoggedAnalyticsEvents(),
				"the notice run must not reach analytics logging")
			require.Empty(t, harness.server.CliTelemetry.Instances)
			require.FileExists(t, harness.noticeMarkerPath())
		})
	}
}

func TestAnalyticsNoticeOnFirstCommand(t *testing.T) {
	t.Run("first invocation discloses analytics without logging an analytics event", func(t *testing.T) {
		harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
			loggingEnabled:      true,
			noticeMarkerPresent: false,
			stderrTTY:           true,
		})

		first := harness.execute("ping")

		require.True(t, first.AnalyticsEligible)
		require.True(t, first.AnalyticsNoticeEligible)
		require.Contains(t, ansi.Strip(first.Stderr), "The Render CLI collects usage data")
		require.Zero(t, first.countLoggedAnalyticsEvents(),
			"the run that discloses must not reach analytics logging")
		require.FileExists(t, harness.noticeMarkerPath())

		second := harness.execute("ping")

		require.NotContains(t, ansi.Strip(second.Stderr), "The Render CLI collects usage data",
			"the marker must suppress a repeat notice")
		require.Equal(t, 1, second.countLoggedAnalyticsEvents(),
			"the run after disclosure should log one analytics event")
	})

	t.Run("does not pollute CI output with analytics disclosure", func(t *testing.T) {
		harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
			loggingEnabled:      true,
			noticeMarkerPresent: false,
			stderrTTY:           true,
			ci:                  true,
		})

		result := harness.execute("ping")

		require.NotContains(t, ansi.Strip(result.Stderr), "The Render CLI collects usage data")
		require.Equal(t, 1, result.countLoggedAnalyticsEvents(),
			"CI should bypass the disclosure gate and log one analytics event")
		require.NoFileExists(t, harness.noticeMarkerPath(),
			"a CI run must leave the notice for the first run with a human at the terminal")
	})
}

func TestAnalyticsNoticeOnFirstHelpCommand(t *testing.T) {
	harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
		noticeMarkerPresent: false,
		stderrTTY:           true,
		allowSubprocess:     true,
	})
	require.NoFileExists(t, harness.noticeMarkerPath(),
		"the test must start without a marker or its first-help assertions are unsound")

	first := harness.execute("--help")

	require.Equal(t, 0, first.ExitCode)
	require.True(t, first.AnalyticsNoticeEligible)
	require.Contains(t, ansi.Strip(first.Stderr), "The Render CLI collects usage data")
	require.Empty(t, harness.server.CliTelemetry.Instances,
		"help must not emit analytics before the disclosure")
	require.FileExists(t, harness.noticeMarkerPath())

	second := harness.execute("--help")

	require.Equal(t, 0, second.ExitCode)
	require.Empty(t, second.Stderr)
	require.Len(t, harness.server.CliTelemetry.Instances, 1,
		"help after the disclosure should emit analytics")
}

// TestAnalyticsNoticeOnFirstAnalyticsIneligibleUserCommand verifies that a
// user-facing command can show the notice even when its own execution is
// excluded from analytics.
func TestAnalyticsNoticeOnFirstAnalyticsIneligibleUserCommand(t *testing.T) {
	testCases := []struct {
		name string
		args []string
	}{
		{name: "login", args: []string{"login", "--help"}},
		{name: "logout", args: []string{"logout"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
				noticeMarkerPresent: false,
				stderrTTY:           true,
				allowSubprocess:     true,
			})

			result := harness.execute(tc.args...)

			require.False(t, result.AnalyticsEligible)
			require.True(t, result.AnalyticsNoticeEligible)
			require.Contains(t, ansi.Strip(result.Stderr), "The Render CLI collects usage data")
			require.FileExists(t, harness.noticeMarkerPath())
			require.Empty(t, harness.server.CliTelemetry.Instances)
		})
	}
}

func TestShellCompletionDoesNotPerformDisclosure(t *testing.T) {
	testCases := []struct {
		name string
		args []string
	}{
		{name: "completion command", args: []string{"completion"}},
		{name: "bash completion script", args: []string{"completion", "bash"}},
		{name: "zsh completion script", args: []string{"completion", "zsh"}},
		{name: "completion request", args: []string{cobra.ShellCompRequestCmd, ""}},
		{name: "completion request without descriptions", args: []string{cobra.ShellCompNoDescRequestCmd, ""}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
				loggingEnabled:      true,
				noticeMarkerPresent: false,
				stderrTTY:           true,
			})

			result := harness.execute(tc.args...)

			require.Equal(t, 0, result.ExitCode)
			require.False(t, result.AnalyticsEligible)
			require.False(t, result.AnalyticsNoticeEligible)
			require.NotContains(t, ansi.Strip(result.Stderr), "The Render CLI collects usage data")
			require.Zero(t, result.countLoggedAnalyticsEvents(),
				"shell completion must not reach analytics logging")
			require.NoFileExists(t, harness.noticeMarkerPath())
		})
	}
}

// TestHiddenUserCommandPerformsDisclosure verifies that being hidden from help
// does not make a command notice-ineligible. A user may still directly invoke
// a hidden command, such as one retained for backward compatibility.
func TestHiddenUserCommandPerformsDisclosure(t *testing.T) {
	harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
		noticeMarkerPresent: false,
		stderrTTY:           true,
	})

	result := harness.execute("hidden-user-command")

	require.True(t, result.AnalyticsNoticeEligible,
		"hidden commands remain notice-eligible unless explicitly marked otherwise")
	require.Contains(t, ansi.Strip(result.Stderr), "The Render CLI collects usage data")
	require.FileExists(t, harness.noticeMarkerPath())
}

// `render analytics notice` calls analyticsnotice.ShowIfNeeded directly, while
// the completion callback does the same for notice-eligible executions. A
// failed marker write verifies that the command's notice ineligibility prevents
// the completion callback from printing the disclosure a second time.
func TestAnalyticsNoticeCommandDisclosesExactlyOnce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not enforce the POSIX directory permissions this test relies on")
	}

	harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
		noticeMarkerPresent: false,
		stderrTTY:           true,
	})
	// Read and execute but not write: the marker's directory can be listed, so
	// the notice still believes it has never been shown, but writing the marker
	// afterwards fails.
	require.NoError(t, os.MkdirAll(filepath.Join(harness.configDir, "state"), 0o500),
		"the test needs a state directory the marker cannot be written into")

	result := harness.execute("analytics", "notice")

	require.Equal(t, 1,
		strings.Count(ansi.Strip(result.Stderr), "The Render CLI collects usage data"),
		"The disclosure must be printed to stderr exactly once")
}

func TestAnalyticsSendDoesNotPerformDisclosure(t *testing.T) {
	harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
		noticeMarkerPresent: false,
		stderrTTY:           true,
	})

	// The event file does not exist, so the send itself fails. This test concerns
	// only whether completion handling treats the transport as user-facing.
	result := harness.execute("analytics", "send", filepath.Join(harness.configDir, "no-such-event.json"))

	require.NotNil(t, result.OutputFormat, "the transport command must still receive normal pre-run processing")
	// `render analytics send` should not print the analytics notice as its output
	// is normally invisible to users.
	require.Empty(t, result.Stderr)
	require.NoFileExists(t, harness.noticeMarkerPath())
}

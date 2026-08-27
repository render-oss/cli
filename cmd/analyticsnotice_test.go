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

	"github.com/render-oss/cli/internal/files"
	"github.com/render-oss/cli/pkg/analytics"
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
	for _, args := range [][]string{{"analytics", "--help"}, {"help", "analytics"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			harness := newAnalyticsHarness(t, false)
			harness.forceStderrTTY()

			result := harness.execute(args...)

			require.Equal(t, 0, result.ExitCode)
			require.False(t, result.AnalyticsNoticeEligible)
			require.NotContains(t, harness.stdout.String(), "notice")
			require.NoFileExists(t, harness.analyticsNoticeMarkerPath())
		})
	}
}

func TestAnalyticsNoticeIsAnalyticsIneligible(t *testing.T) {
	harness := newAnalyticsHarness(t, true)

	result := harness.execute("analytics", "notice")

	require.False(t, result.AnalyticsEligible,
		"showing the disclosure is not user activity, and reporting it would send telemetry before the user has had any chance to opt out")
	require.False(t, result.AnalyticsNoticeEligible)
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
	testCases := []struct {
		name             string
		args             []string
		wantLoggedEvents int
	}{
		{name: "regular command", args: []string{"ping"}, wantLoggedEvents: 1},
		{name: "explicit notice command", args: []string{"analytics", "notice"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RENDER_LOG_ANALYTICS", "1")
			harness := newAnalyticsHarness(t, true)
			t.Setenv("RENDER_TEST_ENABLE_ANALYTICS", "")
			// Reload after closing the gate so the Sender has logging on but POSTing off.
			harness.reloadDependencies()
			harness.forceStderrTTY()

			result := harness.execute(tc.args...)

			require.Equal(t, 0, result.ExitCode)
			require.NotContains(t, ansi.Strip(harness.stderr.String()), "The Render CLI collects usage data")
			require.Equal(t, tc.wantLoggedEvents,
				harness.countLoggedAnalyticsEvents(result.CommandPath),
				"analytics logging should follow command eligibility while the development gate is closed")
			require.Empty(t, harness.server.CliTelemetry.Instances)
			require.NoFileExists(t, harness.analyticsNoticeMarkerPath(),
				"a disabled rollout must leave the notice for the eventual rollout")
		})
	}
}

func TestAnalyticsNoticeOnFirstCommand(t *testing.T) {
	t.Run("first invocation discloses analytics without logging an analytics event", func(t *testing.T) {
		// Logging makes reaching the analytics sender observable on stderr. The
		// sender reads this setting when it is constructed, so it has to be set
		// before the harness is built.
		// The second run below is the positive control for that ordering, and
		// for the "no event" assertion on the first run.
		t.Setenv("RENDER_LOG_ANALYTICS", "1")
		harness := newAnalyticsHarness(t, true)
		harness.forceStderrTTY()

		first := harness.execute("ping")

		require.True(t, first.AnalyticsEligible)
		require.True(t, first.AnalyticsNoticeEligible)
		require.Contains(t, ansi.Strip(harness.stderr.String()), "The Render CLI collects usage data")
		require.Zero(t,
			harness.countLoggedAnalyticsEvents(first.CommandPath),
			"the run that discloses must not reach analytics logging")
		require.FileExists(t, harness.analyticsNoticeMarkerPath())

		harness.stderr.Reset()
		second := harness.execute("ping")

		require.NotContains(t, ansi.Strip(harness.stderr.String()), "The Render CLI collects usage data",
			"the marker must suppress a repeat notice")
		require.Equal(t, 1,
			harness.countLoggedAnalyticsEvents(second.CommandPath),
			"the run after disclosure should log one analytics event")
	})

	t.Run("does not pollute CI output with analytics disclosure", func(t *testing.T) {
		t.Setenv("RENDER_LOG_ANALYTICS", "1")
		harness := newAnalyticsHarness(t, true)
		harness.deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
			return command.RuntimeSignals{CI: true, StderrTTY: true}, nil
		}

		result := harness.execute("ping")

		require.NotContains(t, ansi.Strip(harness.stderr.String()), "The Render CLI collects usage data")
		require.Equal(t, 1,
			harness.countLoggedAnalyticsEvents(result.CommandPath),
			"CI should bypass the disclosure gate and log one analytics event")
		require.NoFileExists(t, harness.analyticsNoticeMarkerPath(),
			"a CI run must leave the notice for the first run with a human at the terminal")
	})
}

func TestAnalyticsNoticeOnFirstHelpCommand(t *testing.T) {
	t.Setenv("RENDER_LOG_ANALYTICS", "")
	harness := newAnalyticsHarness(t, true)
	harness.forceStderrTTY()
	// The analytics subprocess creates its own default client, so it needs the
	// fake server's URL through the production RENDER_HOST configuration path.
	t.Setenv("RENDER_HOST", harness.server.URL()+"/")
	t.Setenv("RENDER_CLI_ANALYTICS_STRATEGY", "sync")
	t.Setenv(analytics.AllowSubprocessInTestsEnv, "1")
	require.NoFileExists(t, harness.analyticsNoticeMarkerPath(),
		"the test must start without a marker or its first-help assertions are unsound")

	first := harness.execute("--help")

	require.Equal(t, 0, first.ExitCode)
	require.True(t, first.AnalyticsNoticeEligible)
	require.Contains(t, ansi.Strip(harness.stderr.String()), "The Render CLI collects usage data")
	require.Empty(t, harness.server.CliTelemetry.Instances,
		"help must not emit analytics before the disclosure")
	require.FileExists(t, harness.analyticsNoticeMarkerPath())

	harness.stderr.Reset()
	second := harness.execute("--help")

	require.Equal(t, 0, second.ExitCode)
	require.Empty(t, harness.stderr.String())
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
			harness := newAnalyticsHarness(t, true)
			harness.forceStderrTTY()

			result := harness.execute(tc.args...)

			require.False(t, result.AnalyticsEligible)
			require.True(t, result.AnalyticsNoticeEligible)
			require.Contains(t, ansi.Strip(harness.stderr.String()), "The Render CLI collects usage data")
			require.FileExists(t, harness.analyticsNoticeMarkerPath())
			require.Empty(t, harness.server.CliTelemetry.Instances)
		})
	}
}

func TestSkipAnalyticsSendDoesNotSuppressNotice(t *testing.T) {
	t.Setenv("RENDER_LOG_ANALYTICS", "1")
	harness := newAnalyticsHarness(t, true)
	harness.forceStderrTTY()
	result := command.ExecutionResult{
		AnalyticsEligible:       true,
		AnalyticsNoticeEligible: true,
		CommandPath:             "render future-command",
		CompletionKind:          command.CompletionKindSuccess,
		SkipAnalyticsSend:       true,
	}

	onExecutionComplete(result, harness.deps, harness.root)

	require.Contains(t, ansi.Strip(harness.stderr.String()), "The Render CLI collects usage data")
	require.FileExists(t, harness.analyticsNoticeMarkerPath())
	require.Zero(t,
		harness.countLoggedAnalyticsEvents(result.CommandPath),
		"a skipped execution must not reach analytics logging")
	require.Empty(t, harness.server.CliTelemetry.Instances)
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
			t.Setenv("RENDER_LOG_ANALYTICS", "1")
			harness := newAnalyticsHarness(t, true)
			harness.forceStderrTTY()

			result := harness.execute(tc.args...)

			require.Equal(t, 0, result.ExitCode)
			require.False(t, result.AnalyticsEligible)
			require.False(t, result.AnalyticsNoticeEligible)
			require.NotContains(t, ansi.Strip(harness.stderr.String()), "The Render CLI collects usage data")
			require.Zero(t,
				harness.countLoggedAnalyticsEvents(result.CommandPath),
				"shell completion must not reach analytics logging")
			require.NoFileExists(t, harness.analyticsNoticeMarkerPath())
		})
	}
}

// TestHiddenUserCommandPerformsDisclosure verifies that being hidden from help
// does not make a command notice-ineligible. A user may still directly invoke
// a hidden command, such as one retained for backward compatibility.
func TestHiddenUserCommandPerformsDisclosure(t *testing.T) {
	harness := newAnalyticsHarness(t, true)
	harness.forceStderrTTY()
	harness.root.AddCommand(&cobra.Command{
		Use:    "hidden-user-command",
		Hidden: true,
		RunE:   func(*cobra.Command, []string) error { return nil },
	})

	result := harness.execute("hidden-user-command")

	require.True(t, result.AnalyticsNoticeEligible,
		"hidden commands remain notice-eligible unless explicitly marked otherwise")
	require.Contains(t, ansi.Strip(harness.stderr.String()), "The Render CLI collects usage data")
	require.FileExists(t, harness.analyticsNoticeMarkerPath())
}

// `render analytics notice` calls analyticsnotice.ShowIfNeeded directly, while
// the completion callback does the same for notice-eligible executions. A
// failed marker write verifies that the command's notice ineligibility prevents
// the completion callback from printing the disclosure a second time.
func TestAnalyticsNoticeCommandDisclosesExactlyOnce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not enforce the POSIX directory permissions this test relies on")
	}

	harness := newAnalyticsHarness(t, true)
	harness.forceStderrTTY()
	// Read and execute but not write: the marker's directory can be listed, so
	// the notice still believes it has never been shown, but writing the marker
	// afterwards fails.
	require.NoError(t, os.MkdirAll(filepath.Join(harness.configDir, "state"), 0o500),
		"the test needs a state directory the marker cannot be written into")

	harness.execute("analytics", "notice")

	require.Equal(t, 1,
		strings.Count(ansi.Strip(harness.stderr.String()), "The Render CLI collects usage data"),
		"The disclosure must be printed to stderr exactly once")
}

func TestAnalyticsSendDoesNotPerformDisclosure(t *testing.T) {
	harness := newAnalyticsHarness(t, true)
	harness.forceStderrTTY()

	// The event file does not exist, so the send itself fails. This test concerns
	// only whether completion handling treats the transport as user-facing.
	result := harness.execute("analytics", "send", filepath.Join(harness.configDir, "no-such-event.json"))

	require.NotNil(t, result.OutputFormat, "the transport command must still receive normal pre-run processing")
	// `render analytics send` should not print the analytics notice as its output
	// is normally invisible to users.
	require.Empty(t, harness.stderr.String())
	require.NoFileExists(t, harness.analyticsNoticeMarkerPath())
}

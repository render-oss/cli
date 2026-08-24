package analyticsnotice

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/testassert"
	"github.com/render-oss/cli/pkg/analytics"
	"github.com/render-oss/cli/pkg/command"
)

func clearColorEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE"} {
		t.Setenv(key, "")
	}
}

func TestBuildNotice(t *testing.T) {
	clearColorEnvironment(t)

	var buf bytes.Buffer
	s := command.NewStream(&buf)

	t.Run("purple padded box frames the notice", func(t *testing.T) {
		lines := strings.Split(ansi.Strip(buildNotice(s, analytics.OptOutReasonNone)), "\n")

		require.True(t, strings.HasPrefix(lines[0], "╭"), "top-left corner")
		require.True(t, strings.HasSuffix(lines[0], "╮"), "top-right corner")
		require.True(t, strings.HasPrefix(lines[len(lines)-1], "╰"), "bottom-left corner")
		require.True(t, strings.HasSuffix(lines[len(lines)-1], "╯"), "bottom-right corner")
		for _, line := range lines[1 : len(lines)-1] {
			require.True(t, strings.HasPrefix(line, "│"), "left border: %q", line)
			require.True(t, strings.HasSuffix(line, "│"), "right border: %q", line)
		}
		require.True(t, strings.HasPrefix(lines[2], "│  Welcome to the Render CLI!"), "horizontal padding")
		require.Empty(t, strings.Trim(lines[1], " │"), "top padding")
		require.Empty(t, strings.Trim(lines[len(lines)-2], " │"), "bottom padding")
	})

	t.Run("telemetry on explains collection and how to opt out", func(t *testing.T) {
		out := ansi.Strip(buildNotice(s, analytics.OptOutReasonNone))

		testassert.ContainsInOrder(t, out,
			"Welcome to the Render CLI!",
			"To get started, run", "render login",
			"Learn more at https://render.com/docs/cli.",
			"The Render CLI collects usage data",
			"Nothing has been sent yet.",
			"Opt out: set DO_NOT_TRACK=1 in your environment",
			telemetryDocsURL,
		)
		require.NotContains(t, out, "anonymous")
	})

	t.Run("opted out shows the confirmation instead of the invitation to opt out", func(t *testing.T) {
		out := ansi.Strip(buildNotice(s, analytics.OptOutReasonDoNotTrack))

		testassert.ContainsInOrder(t, out,
			"Welcome to the Render CLI!",
			"Telemetry is disabled due to your DO_NOT_TRACK setting.",
			telemetryDocsURL,
		)
		require.NotContains(t, out, "Opt out:")
		require.NotContains(t, out, "collects usage data")
	})

	t.Run("wraps the boxed notice to fit a narrow terminal", func(t *testing.T) {
		const terminalWidth = 30
		out := ansi.Strip(buildNoticeAtWidth(s.Renderer(), terminalWidth, analytics.OptOutReasonNone))

		for _, line := range strings.Split(out, "\n") {
			require.Equal(t, terminalWidth, ansi.StringWidth(line), "line exceeds terminal width: %q", line)
			require.True(t, strings.HasPrefix(line, "│") || strings.HasPrefix(line, "╭") || strings.HasPrefix(line, "╰"), "missing left border: %q", line)
			require.True(t, strings.HasSuffix(line, "│") || strings.HasSuffix(line, "╮") || strings.HasSuffix(line, "╯"), "missing right border: %q", line)
		}
		require.Contains(t, out, "telemetry", "wrapped details URL retains its tail")
	})

	t.Run("reason mapping covers every opt-out mechanism", func(t *testing.T) {
		testCases := []struct {
			reason analytics.OptOutReason
			want   string
		}{
			{analytics.OptOutReasonDoNotTrack, "Telemetry is disabled due to your DO_NOT_TRACK setting."},
			{analytics.OptOutReasonDisableAnalyticsEnv, "Telemetry is disabled due to your RENDER_CLI_DISABLE_ANALYTICS setting."},
			{analytics.OptOutReasonAnalyticsDisabledConfig, "Telemetry is disabled due to the analytics.disabled setting in your Render CLI config file."},
			{"something-new", "Telemetry is disabled due to your settings."},
		}
		for _, tc := range testCases {
			require.Contains(t, ansi.Strip(buildNotice(s, tc.reason)), tc.want)
		}
	})
}

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

	t.Run("padded block of uniform width, no border", func(t *testing.T) {
		lines := strings.Split(ansi.Strip(buildNotice(s, analytics.OptOutReasonNone)), "\n")

		width := ansi.StringWidth(lines[0])
		for _, line := range lines {
			require.Equal(t, width, ansi.StringWidth(line),
				"every line must fill the block so the background reads as a rectangle: %q", line)
			require.NotContains(t, line, "│", "the block is a background, not a border")
		}
		require.Empty(t, strings.TrimSpace(lines[0]), "top padding")
		require.Empty(t, strings.TrimSpace(lines[len(lines)-1]), "bottom padding")
		require.True(t, strings.HasPrefix(lines[1], "  The Render CLI collects"), "horizontal padding")
	})

	t.Run("one blank line separates what we collect from what to do about it", func(t *testing.T) {
		lines := strings.Split(ansi.Strip(buildNotice(s, analytics.OptOutReasonNone)), "\n")

		// Drop the padding rows the style adds above and below the content.
		body := lines[1 : len(lines)-1]
		var blanks []int
		for i, line := range body {
			if strings.TrimSpace(line) == "" {
				blanks = append(blanks, i)
			}
		}
		require.Len(t, blanks, 1, "exactly one blank line, and it is in the middle:\n%s", strings.Join(body, "\n"))
	})

	t.Run("telemetry on explains collection and how to opt out", func(t *testing.T) {
		out := ansi.Strip(buildNotice(s, analytics.OptOutReasonNone))

		// Fragments, not whole sentences: the box wraps to the terminal width.
		testassert.ContainsInOrder(t, out,
			"The Render CLI collects usage data",
			"No data has been sent to Render yet.",
			"To opt out, set DO_NOT_TRACK=1",
			telemetryDocsURL,
		)
		require.NotContains(t, out, "anonymous")
	})

	t.Run("opted out still discloses, and says how to turn telemetry on", func(t *testing.T) {
		out := ansi.Strip(buildNotice(s, analytics.OptOutReasonDoNotTrack))

		testassert.ContainsInOrder(t, out,
			"The Render CLI collects usage data",
			"Telemetry is currently disabled",
			"To enable telemetry, remove DO_NOT_TRACK",
			telemetryDocsURL,
		)
		require.NotContains(t, out, "To opt out,")
	})

	t.Run("wraps the boxed notice to fit a narrow terminal", func(t *testing.T) {
		const terminalWidth = 30
		out := ansi.Strip(buildNoticeAtWidth(s.Renderer(), terminalWidth, analytics.OptOutReasonNone))

		for _, line := range strings.Split(out, "\n") {
			require.Equal(t, terminalWidth, ansi.StringWidth(line), "line does not fill the terminal width: %q", line)
		}
		require.Contains(t, out, "telemetry", "wrapped details URL retains its tail")
	})

	t.Run("every opt-out mechanism says how to undo itself", func(t *testing.T) {
		testCases := []struct {
			reason analytics.OptOutReason
			want   string
		}{
			{analytics.OptOutReasonDoNotTrack, "To enable telemetry, remove DO_NOT_TRACK from your environment."},
			{analytics.OptOutReasonDisableAnalyticsEnv, "remove RENDER_CLI_DISABLE_ANALYTICS from your"},
			{"something-new", "check your Render CLI settings."},
		}
		for _, tc := range testCases {
			require.Contains(t, ansi.Strip(buildNotice(s, tc.reason)), tc.want)
		}
	})
}

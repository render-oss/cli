package analyticsnotice

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/render-oss/cli/pkg/analytics"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/style"
)

// collectionDisclosure should be the first sentence of every notice variant produced by this package
const collectionDisclosure = "The Render CLI collects usage data on commands and performance to help us keep improving."

const (
	telemetryDocsURL = "https://render.com/docs/cli#telemetry"
	// maxNoticeWidth fits the longest line, the opening disclosure, on one row
	// once the horizontal padding is taken off. Terminals narrower than this
	// still wrap: the cap is a ceiling, not a floor.
	maxNoticeWidth = 93
)

// buildNotice returns the analytics disclosure ready to print, as a padded
// block on a tinted background. The renderer derives its color profile from s.
//
// [analytics.OptOutReasonNone] means telemetry is on and the disclosure is
// shown as usual; any other value replaces the disclosure with the opted-out
// confirmation naming the mechanism that applies.
func buildNotice(s *command.Stream, optOutReason analytics.OptOutReason) string {
	return buildNoticeAtWidth(s.Renderer(), s.Width(), optOutReason)
}

// buildNoticeAtWidth renders the notice for a terminal width. Width zero means
// unknown, so the content uses its natural width.
func buildNoticeAtWidth(renderer *lipgloss.Renderer, width int, optOutReason analytics.OptOutReason) string {
	noticeStyle := renderer.NewStyle().
		Background(style.ColorSubtleBackground).
		Foreground(style.ColorFocus).
		Padding(1, 2)
	if width > 0 {
		noticeStyle = noticeStyle.Width(min(width, maxNoticeWidth))
	}

	return noticeStyle.Render(strings.Join(telemetrySection(optOutReason), "\n"))
}

// telemetrySection returns the telemetry notice to show the user.
func telemetrySection(optOutReason analytics.OptOutReason) []string {
	if optOutReason == analytics.OptOutReasonNone {
		return []string{
			collectionDisclosure,
			"No data has been sent to Render yet.",
			"",
			"To opt out, set DO_NOT_TRACK=1 in your environment.",
			"Learn more about telemetry: " + telemetryDocsURL,
		}
	}

	return []string{
		collectionDisclosure,
		"Telemetry is currently disabled, so no data is being sent to Render.",
		"",
		enableInstruction(optOutReason),
		"Learn more about telemetry: " + telemetryDocsURL,
	}
}

// enableInstruction tells the user how to undo the opt-out currently in effect.
// If the user has not opted out ([analytics.OptOutReasonNone]), then you should not call this method.
func enableInstruction(optOutReason analytics.OptOutReason) string {
	//exhaustive:enforce
	switch optOutReason {
	case analytics.OptOutReasonDoNotTrack:
		return "To enable telemetry, remove DO_NOT_TRACK from your environment."
	case analytics.OptOutReasonDisableAnalyticsEnv:
		return "To enable telemetry, remove RENDER_CLI_DISABLE_ANALYTICS from your environment."
	case analytics.OptOutReasonAnalyticsDisabledConfig:
		return "To enable telemetry, remove analytics.disabled from your Render CLI config file."
	case analytics.OptOutReasonNone:
		// telemetrySection handles the not-opted-out case before calling this.
		return "To enable telemetry, check your Render CLI settings."
	default:
		// Keep telemetry setup best-effort if a caller constructs an invalid value.
		return "To enable telemetry, check your Render CLI settings."
	}
}

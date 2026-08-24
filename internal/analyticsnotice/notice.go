package analyticsnotice

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/render-oss/cli/pkg/analytics"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/style"
)

const (
	telemetryDocsURL = "https://render.com/docs/cli/telemetry"
	maxNoticeWidth   = 76
	boxBorderColumns = 2
)

// buildNotice returns the welcome notice ready to print. It contains the welcome
// and get-started copy followed by the telemetry disclosure, framed by a
// Render-purple box. The renderer derives its color profile from s.
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
	var notice strings.Builder
	notice.WriteString(welcomeSection(renderer))
	notice.WriteString("\n\n")
	notice.WriteString(strings.Join(telemetrySection(optOutReason), "\n"))

	noticeStyle := renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.ColorInfo).
		Padding(1, 2)
	if width > 0 {
		noticeStyle = noticeStyle.Width(min(width, maxNoticeWidth) - boxBorderColumns)
	}

	return noticeStyle.Render(notice.String())
}

// welcomeSection is the greeting and first step for a new user.
func welcomeSection(renderer *lipgloss.Renderer) string {
	title := style.Title.Renderer(renderer)
	commandKey := style.CommandKey.Renderer(renderer)

	return strings.Join([]string{
		title.Render("Welcome to the Render CLI!"),
		"",
		fmt.Sprintf("To get started, run %s.", commandKey.Render("render login")),
		"Learn more at https://render.com/docs/cli.",
	}, "\n")
}

// telemetrySection returns the telemetry notice to show the user.
func telemetrySection(optOutReason analytics.OptOutReason) []string {
	if optOutReason == analytics.OptOutReasonNone {
		return []string{
			"The Render CLI collects usage data to help us improve the product.",
			"Nothing has been sent yet.",
			"",
			"Opt out: set DO_NOT_TRACK=1 in your environment",
			"Details: " + telemetryDocsURL,
		}
	}

	return []string{
		fmt.Sprintf("Telemetry is disabled due to %s.", optOutMechanism(optOutReason)),
		"Details: " + telemetryDocsURL,
	}
}

// optOutMechanism maps an [analytics.OptOutReason] to the phrase used in the notice copy.
// If the user not opted out of analytics ([analytics.OptOutReasonNone]), then you should not call this method.
func optOutMechanism(optOutReason analytics.OptOutReason) string {
	//exhaustive:enforce
	switch optOutReason {
	case analytics.OptOutReasonDoNotTrack:
		return "your DO_NOT_TRACK setting"
	case analytics.OptOutReasonDisableAnalyticsEnv:
		return "your RENDER_CLI_DISABLE_ANALYTICS setting"
	case analytics.OptOutReasonAnalyticsDisabledConfig:
		return "the analytics.disabled setting in your Render CLI config file"
	case analytics.OptOutReasonNone:
		// telemetrySection handles the not-opted-out case before calling this.
		return "your settings"
	default:
		// Keep telemetry setup best-effort if a caller constructs an invalid value.
		return "your settings"
	}
}

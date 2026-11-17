package updatenotice

import (
	"fmt"
	"strings"

	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/style"
)

// Render returns the notice for moving from current to latest as three lines:
// the versions, how to upgrade, and the release notes. The renderer derives its
// color profile from s, so the lines are plain when s is not a terminal.
func Render(s *command.Stream, current, latest string, install Install) string {
	r := s.Renderer()
	heading := r.NewStyle().Foreground(style.ColorWarning).Bold(true).Render("Update available:")
	old := r.NewStyle().Foreground(style.ColorDeprioritized).Render("v" + current)
	arrow := r.NewStyle().Foreground(style.ColorWarning).Render("→")
	changed := r.NewStyle().Foreground(style.ColorOK).Bold(true).Render

	upgrade := "To upgrade, see: " + cfg.InstallationInstructionsURL
	if command := install.upgradeCommand(); command != "" {
		upgrade = "To upgrade, run: " + r.NewStyle().Bold(true).Render(command)
	}

	return fmt.Sprintf("%s %s %s v%s\n%s\nRelease notes: %s",
		heading, old, arrow, highlightChanged(current, latest, changed),
		upgrade, releaseNotesURL(latest))
}

// RenderLine returns the notice as a single uncolored line, for AI agents and
// other readers that are not a person at a terminal.
func RenderLine(current, latest string, install Install) string {
	upgrade := "To upgrade, see: " + cfg.InstallationInstructionsURL
	if command := install.upgradeCommand(); command != "" {
		upgrade = "To upgrade, run: " + command
	}
	return fmt.Sprintf("Update available: v%s → v%s. %s. Release notes: %s",
		current, latest, upgrade, releaseNotesURL(latest))
}

// RenderStatus returns the update status printed by `render --version`: the
// notice when latest is newer than current, or a short confirmation when latest
// is empty because current is already the newest release. With line set, the
// notice is the single line from [RenderLine].
func RenderStatus(s *command.Stream, current, latest string, install Install, line bool) string {
	switch {
	case latest == "":
		return s.Renderer().NewStyle().Foreground(style.ColorOK).Render("You are using the latest version")
	case line:
		return RenderLine(current, latest, install)
	default:
		return Render(s, current, latest, install)
	}
}

func releaseNotesURL(version string) string {
	return cfg.ReleasesURL + "/tag/v" + version
}

// highlightChanged returns latest with each dot-separated component that
// differs from the same component of current passed through highlight, so
// 2.30.5 → 2.33.5 draws attention to "33" alone.
func highlightChanged(current, latest string, highlight func(...string) string) string {
	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")

	for i, part := range latestParts {
		if i >= len(currentParts) || currentParts[i] != part {
			latestParts[i] = highlight(part)
		}
	}
	return strings.Join(latestParts, ".")
}

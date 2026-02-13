package views

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/render-oss/cli/pkg/skills"
	renderstyle "github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/tui"
)

// ── Icon Helpers ─────────────────────────────────────────────────────────────

func iconCheck() string {
	return lipgloss.NewStyle().Foreground(renderstyle.ColorOK).Render("✓")
}

func iconInfo() string {
	return lipgloss.NewStyle().Foreground(renderstyle.ColorInfo).Render("ℹ")
}

func iconWarn() string {
	return lipgloss.NewStyle().Foreground(renderstyle.ColorWarning).Render("⚠")
}

func iconCross() string {
	return lipgloss.NewStyle().Foreground(renderstyle.ColorError).Render("✗")
}

// scopeLabel returns a display string for a skill's scope.
func scopeLabel(scope skills.Scope) string {
	if scope == skills.ScopeProject {
		return "project"
	}
	return "user"
}

// ── Scope Helpers ────────────────────────────────────────────────────────────

// buildScopeForm creates a huh.Form for selecting user vs project scope.
// The selected value is written to the string pointed to by selectedScope.
func buildScopeForm(title string, selectedScope *string) *huh.Form {
	*selectedScope = string(skills.ScopeUser)

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Options(
					huh.NewOption("For me (user scope)", string(skills.ScopeUser)),
					huh.NewOption("For all collaborators on this repository (project scope)", string(skills.ScopeProject)),
				).
				Value(selectedScope),
		),
	)
}

// resolveProjectScope validates and returns the repo root for project scope.
// Returns an error message command if not in a git repository.
func resolveProjectScope() (string, error) {
	repoRoot, err := skills.GetRepoRoot()
	if err != nil {
		return "", fmt.Errorf("project scope requires a git repository")
	}
	return repoRoot, nil
}

// detectInstalledScopes inspects installed skills and returns which scopes are
// present. Skills with no scope set are treated as user scope (legacy state).
func detectInstalledScopes(installed []skills.InstalledSkill) (hasUser, hasProject bool) {
	for _, sk := range installed {
		switch sk.EffectiveScope() {
		case skills.ScopeProject:
			hasProject = true
		default:
			hasUser = true
		}
	}
	return
}

// handleScopeFormUpdate processes form updates for scope selection and returns
// the completed scope, repo root, and whether the form is done.
// Returns (scope, repoRoot, done, abortCmd).
func handleScopeFormUpdate(form *huh.Form, selectedScope string) (skills.Scope, string, bool, func() tea.Msg) {
	if form.State == huh.StateCompleted {
		scope := skills.Scope(selectedScope)
		if scope == skills.ScopeProject {
			repoRoot, err := resolveProjectScope()
			if err != nil {
				errMsg := func() tea.Msg {
					return tui.ErrorMsg{Err: err}
				}
				return scope, "", true, errMsg
			}
			return scope, repoRoot, true, nil
		}
		return scope, "", true, nil
	}
	return "", "", false, nil
}

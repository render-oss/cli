package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/render-oss/cli/pkg/skills"
	renderstyle "github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/tui"
)

// ── Remove step state machine ───────────────────────────────────────────────

type removeStep int

const (
	removeStepLoading removeStep = iota
	removeStepSelectScope
	removeStepSelect
	removeStepRemoving
	removeStepDone
)

// ── Messages ────────────────────────────────────────────────────────────────

type skillsRemoveStateLoadedMsg struct {
	state         *skills.SkillsState
	detectedTools []skills.Tool
	warnings      []string
}

type skillsRemoveDoneMsg struct {
	successCount int
	errors       []string
	remaining    []skills.InstalledSkill
}

// ── View ────────────────────────────────────────────────────────────────────

// SkillsRemoveView implements the interactive skills remove flow.
type SkillsRemoveView struct {
	step removeStep

	// Forms
	scopeForm  *huh.Form
	selectForm *huh.Form

	// Scope selection
	selectedScope string // "user" or "project"
	scope         skills.Scope
	repoRoot      string

	// State
	state         *skills.SkillsState
	detectedTools []skills.Tool
	selectedTools []skills.Tool

	// Selection
	toRemoveNames []string

	// Results
	successCount int
	removeErrors []string
	remaining    []skills.InstalledSkill

	// Final result
	doneMessage string

	// Status
	statusLines []string

	// Sizing
	width  int
	height int
}

func NewSkillsRemoveView(scope skills.Scope) *SkillsRemoveView {
	return &SkillsRemoveView{
		step:  removeStepLoading,
		scope: scope,
	}
}

func (v *SkillsRemoveView) Init() tea.Cmd {
	return v.loadStateCmd()
}

// ── Update ──────────────────────────────────────────────────────────────────

func (v *SkillsRemoveView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tui.StackSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		if v.scopeForm != nil {
			v.scopeForm = v.scopeForm.WithWidth(msg.Width).WithHeight(msg.Height)
		}
		if v.selectForm != nil {
			v.selectForm = v.selectForm.WithWidth(msg.Width).WithHeight(msg.Height)
		}
		return v, nil
	}

	switch v.step {
	case removeStepLoading:
		return v.updateLoading(msg)
	case removeStepSelectScope:
		return v.updateSelectScope(msg)
	case removeStepSelect:
		return v.updateSelect(msg)
	case removeStepRemoving:
		return v.updateRemoving(msg)
	case removeStepDone:
		return v, nil
	}
	return v, nil
}

func (v *SkillsRemoveView) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case skillsRemoveStateLoadedMsg:
		v.state = msg.state
		v.detectedTools = msg.detectedTools

		for _, w := range msg.warnings {
			v.addStatus("  %s %s", iconWarn(), w)
		}

		// Check for empty skills before scope selection
		if len(v.state.Skills) == 0 && !v.state.HasSelections() {
			v.doneMessage = "No installed skills found."
			v.step = removeStepDone
			return v, nil
		}

		// If scope was pre-selected via --scope flag, resolve and proceed
		if v.scope != "" {
			if v.scope == skills.ScopeProject {
				repoRoot, err := resolveProjectScope()
				if err != nil {
					return v, func() tea.Msg {
						return tui.ErrorMsg{Err: err}
					}
				}
				v.repoRoot = repoRoot
			}
			return v.proceedAfterScopeSelection()
		}

		// Auto-detect scope from installed skills
		hasUser, hasProject := detectInstalledScopes(v.state.Skills)
		if hasUser && !hasProject {
			v.scope = skills.ScopeUser
			return v.proceedAfterScopeSelection()
		}
		if hasProject && !hasUser {
			v.scope = skills.ScopeProject
			repoRoot, err := resolveProjectScope()
			if err != nil {
				return v, func() tea.Msg {
					return tui.ErrorMsg{Err: err}
				}
			}
			v.repoRoot = repoRoot
			return v.proceedAfterScopeSelection()
		}

		// Skills at both scopes — show picker
		v.scopeForm = buildScopeForm("Where do you want to remove skills from?", &v.selectedScope)
		v.step = removeStepSelectScope
		return v, v.scopeForm.Init()
	}
	return v, nil
}

func (v *SkillsRemoveView) updateSelectScope(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := v.scopeForm.Update(msg)
	if m, ok := form.(*huh.Form); ok {
		v.scopeForm = m
	}

	scope, repoRoot, done, errCmd := handleScopeFormUpdate(v.scopeForm, v.selectedScope)
	if errCmd != nil {
		return v, errCmd
	}
	if done {
		v.scope = scope
		v.repoRoot = repoRoot
		return v.proceedAfterScopeSelection()
	}
	if v.scopeForm.State == huh.StateAborted {
		return v, tea.Quit
	}
	return v, cmd
}

func (v *SkillsRemoveView) proceedAfterScopeSelection() (tea.Model, tea.Cmd) {
	if !v.state.HasSelections() || len(v.skillsForScope()) == 0 {
		v.doneMessage = fmt.Sprintf("No installed skills found at %s scope.", v.scope)
		v.step = removeStepDone
		return v, nil
	}

	v.buildSelectedTools()
	if len(v.selectedTools) == 0 {
		return v, func() tea.Msg {
			return tui.ErrorMsg{Err: fmt.Errorf("no matching tools found")}
		}
	}

	v.buildSelectForm()
	v.step = removeStepSelect
	return v, v.selectForm.Init()
}

func (v *SkillsRemoveView) updateSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := v.selectForm.Update(msg)
	if m, ok := form.(*huh.Form); ok {
		v.selectForm = m
	}

	if v.selectForm.State == huh.StateCompleted {
		if len(v.toRemoveNames) == 0 {
			v.doneMessage = "No skills selected for removal."
			v.step = removeStepDone
			return v, nil
		}
		v.step = removeStepRemoving
		return v, v.removeCmd()
	}
	if v.selectForm.State == huh.StateAborted {
		return v, tea.Quit
	}
	return v, cmd
}

func (v *SkillsRemoveView) updateRemoving(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case skillsRemoveDoneMsg:
		v.successCount = msg.successCount
		v.removeErrors = msg.errors
		v.remaining = msg.remaining
		v.step = removeStepDone

		if v.successCount == 0 {
			return v, func() tea.Msg {
				return tui.ErrorMsg{Err: fmt.Errorf("failed to remove skills from any tool")}
			}
		}

		v.doneMessage = v.summaryText()
		return v, nil
	}
	return v, nil
}

// ── View ────────────────────────────────────────────────────────────────────

func (v *SkillsRemoveView) View() string {
	var sb strings.Builder

	for _, line := range v.statusLines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if len(v.statusLines) > 0 {
		sb.WriteString("\n")
	}

	switch v.step {
	case removeStepSelectScope:
		if v.scopeForm != nil {
			sb.WriteString(v.scopeForm.View())
		}
	case removeStepSelect:
		if v.selectForm != nil {
			sb.WriteString(v.selectForm.View())
		}
	case removeStepDone:
		if v.doneMessage != "" {
			sb.WriteString(v.doneMessage)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// ── Commands ────────────────────────────────────────────────────────────────

func (v *SkillsRemoveView) loadStateCmd() tea.Cmd {
	return func() tea.Msg {
		return tui.LoadingDataMsg{
			LoadingMsgTmpl: "%s Loading installed skills...",
			Cmd: tea.Sequence(
				func() tea.Msg {
					loaded, err := skills.LoadOrRebuildState()
					if err != nil {
						return tui.ErrorMsg{Err: err}
					}

					return skillsRemoveStateLoadedMsg{
						state:         loaded.State,
						detectedTools: loaded.DetectedTools,
						warnings:      loaded.Warnings,
					}
				},
				func() tea.Msg { return tui.DoneLoadingDataMsg{} },
			),
		}
	}
}

func (v *SkillsRemoveView) removeCmd() tea.Cmd {
	selectedTools := v.selectedTools
	toRemove := v.toRemoveNames
	state := v.state
	scope := v.scope
	repoRoot := v.repoRoot

	return func() tea.Msg {
		return tui.LoadingDataMsg{
			LoadingMsgTmpl: "%s Removing skills...",
			Cmd: tea.Sequence(
				func() tea.Msg {
					result, err := skills.ExecuteRemoveWithScope(selectedTools, toRemove, state, scope, repoRoot)
					if err != nil {
						return skillsRemoveDoneMsg{
							successCount: 0,
							errors:       []string{err.Error()},
							remaining:    state.Skills,
						}
					}

					return skillsRemoveDoneMsg{
						successCount: len(result.Tools) - len(result.Errors),
						errors:       result.Errors,
						remaining:    state.Skills,
					}
				},
				func() tea.Msg { return tui.DoneLoadingDataMsg{} },
			),
		}
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (v *SkillsRemoveView) addStatus(format string, a ...any) {
	v.statusLines = append(v.statusLines, fmt.Sprintf(format, a...))
}

func (v *SkillsRemoveView) skillsForScope() []skills.InstalledSkill {
	var filtered []skills.InstalledSkill
	for _, sk := range v.state.Skills {
		if sk.EffectiveScope() == v.scope {
			filtered = append(filtered, sk)
		}
	}
	return filtered
}

func (v *SkillsRemoveView) buildSelectedTools() {
	v.selectedTools = skills.IntersectToolsByState(v.detectedTools, v.state)
}

func (v *SkillsRemoveView) buildSelectForm() {
	var options []huh.Option[string]
	for _, sk := range v.state.Skills {
		// Only show skills matching the selected scope.
		if sk.EffectiveScope() != v.scope {
			continue
		}
		label := sk.Name
		if sk.Version != "" && sk.Version != "unknown" {
			label += " (" + sk.Version + ")"
		}
		label += " [" + scopeLabel(sk.EffectiveScope()) + "]"
		// Use EffectiveDirName as the value because RemoveSkills matches
		// against directory names on disk, not frontmatter names.
		options = append(options, huh.NewOption(label, sk.EffectiveDirName()))
	}

	v.selectForm = huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select skills to remove").
				Options(options...).
				Value(&v.toRemoveNames),
		),
	)
}

func (v *SkillsRemoveView) summaryText() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s Skills removed successfully!\n\n", iconCheck()))

	for _, e := range v.removeErrors {
		sb.WriteString(fmt.Sprintf("  %s %s\n", iconCross(), e))
	}

	if len(v.remaining) > 0 {
		dim := lipgloss.NewStyle().Foreground(renderstyle.ColorDeprioritized)
		sb.WriteString("Remaining skills:\n")
		for _, sk := range v.remaining {
			sb.WriteString(fmt.Sprintf("  • %s %s\n", renderstyle.Bold(sk.Name), dim.Render(sk.Version)))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("%s Restart your AI coding tool to apply changes.", iconWarn()))
	return sb.String()
}

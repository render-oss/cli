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

// ── Update step state machine ───────────────────────────────────────────────

type updateStep int

const (
	updateStepLoading updateStep = iota
	updateStepSelectScope
	updateStepCloning
	updateStepChecking
	updateStepSelectUpdates
	updateStepInstalling
	updateStepDone
)

// ── Messages ────────────────────────────────────────────────────────────────

type skillsUpdateStateLoadedMsg struct {
	state         *skills.SkillsState
	detectedTools []skills.Tool
	warnings      []string
}

type skillsUpdateClonedMsg struct {
	tmpDir       string
	remoteSkills []skills.SkillInfo
}

type skillsUpdateCheckMsg struct {
	result       *skills.UpdateCheckResult
	remoteSkills []skills.SkillInfo
}

type skillsUpdateDoneMsg struct {
	successCount int
	errors       []string
}

// ── View ────────────────────────────────────────────────────────────────────

// SkillsUpdateView implements the interactive skills update flow.
type SkillsUpdateView struct {
	step  updateStep
	force bool

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

	// Clone
	tmpDir       string
	remoteSkills []skills.SkillInfo

	// Updates
	outdated      []skills.OutdatedSkill
	toUpdateNames []string
	successCount  int
	updateErrors  []string

	// Final result
	doneMessage string

	// Status
	statusLines []string

	// Sizing
	width  int
	height int
}

func NewSkillsUpdateView(force bool, scope skills.Scope) *SkillsUpdateView {
	return &SkillsUpdateView{
		step:  updateStepLoading,
		force: force,
		scope: scope,
	}
}

func (v *SkillsUpdateView) Init() tea.Cmd {
	return v.loadStateCmd()
}

// ── Update ──────────────────────────────────────────────────────────────────

func (v *SkillsUpdateView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case updateStepLoading:
		return v.updateLoading(msg)
	case updateStepSelectScope:
		return v.updateSelectScope(msg)
	case updateStepCloning:
		return v.updateCloning(msg)
	case updateStepChecking:
		return v.updateChecking(msg)
	case updateStepSelectUpdates:
		return v.updateSelectUpdates(msg)
	case updateStepInstalling:
		return v.updateInstalling(msg)
	case updateStepDone:
		return v, nil
	}
	return v, nil
}

func (v *SkillsUpdateView) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case skillsUpdateStateLoadedMsg:
		v.state = msg.state
		v.detectedTools = msg.detectedTools

		for _, w := range msg.warnings {
			v.addStatus("  %s %s", iconWarn(), w)
		}

		// Check for empty skills before scope selection
		if len(v.state.Skills) == 0 && !v.state.HasSelections() {
			return v, func() tea.Msg {
				return tui.ErrorMsg{Err: fmt.Errorf("no installed skills found. Run render skills install first")}
			}
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
		v.scopeForm = buildScopeForm("Where do you want to update skills?", &v.selectedScope)
		v.step = updateStepSelectScope
		return v, v.scopeForm.Init()
	}
	return v, nil
}

func (v *SkillsUpdateView) updateSelectScope(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (v *SkillsUpdateView) proceedAfterScopeSelection() (tea.Model, tea.Cmd) {
	if !v.state.HasSelections() {
		return v, func() tea.Msg {
			return tui.ErrorMsg{Err: fmt.Errorf("no installed skills found. Run render skills install first")}
		}
	}

	// Intersect saved tools with detected.
	v.buildSelectedTools()
	if len(v.selectedTools) == 0 {
		return v, func() tea.Msg {
			return tui.ErrorMsg{Err: fmt.Errorf("none of the previously selected tools are still installed")}
		}
	}

	v.step = updateStepCloning
	return v, v.cloneRepoCmd()
}

func (v *SkillsUpdateView) updateCloning(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case skillsUpdateClonedMsg:
		v.tmpDir = msg.tmpDir
		v.remoteSkills = msg.remoteSkills

		v.step = updateStepChecking
		return v, v.checkUpdatesCmd()
	}
	return v, nil
}

func (v *SkillsUpdateView) updateChecking(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case skillsUpdateCheckMsg:
		v.outdated = msg.result.Outdated
		v.remoteSkills = msg.remoteSkills

		for _, w := range msg.result.Warnings {
			v.addStatus("  %s %s", iconWarn(), w)
		}
		sl := scopeLabel(v.scope)
		for _, name := range msg.result.UpToDate {
			dim := lipgloss.NewStyle().Foreground(renderstyle.ColorDeprioritized)
			v.addStatus("  %s %s %s [%s]", iconCheck(), renderstyle.Bold(name), dim.Render("(up to date)"), dim.Render(sl))
		}

		if len(v.outdated) == 0 {
			v.saveState()
			v.cleanupTmpDir()
			v.doneMessage = fmt.Sprintf("%s All skills are up to date!", iconCheck())
			v.step = updateStepDone
			return v, nil
		}

		// Show outdated in status.
		dim := lipgloss.NewStyle().Foreground(renderstyle.ColorDeprioritized)
		for _, o := range v.outdated {
			v.addStatus("  %s %s %s [%s]", iconInfo(), renderstyle.Bold(o.Name), dim.Render(o.Label), dim.Render(sl))
		}

		if len(v.outdated) == 1 {
			// Only one outdated — skip the prompt.
			v.toUpdateNames = []string{v.outdated[0].DirName}
			v.step = updateStepInstalling
			return v, v.updateInstallCmd()
		}

		v.buildSelectForm()
		v.step = updateStepSelectUpdates
		return v, v.selectForm.Init()
	}
	return v, nil
}

func (v *SkillsUpdateView) updateSelectUpdates(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := v.selectForm.Update(msg)
	if m, ok := form.(*huh.Form); ok {
		v.selectForm = m
	}

	if v.selectForm.State == huh.StateCompleted {
		if len(v.toUpdateNames) == 0 {
			v.saveState()
			v.cleanupTmpDir()
			v.doneMessage = "No skills selected for update."
			v.step = updateStepDone
			return v, nil
		}
		v.step = updateStepInstalling
		return v, v.updateInstallCmd()
	}
	if v.selectForm.State == huh.StateAborted {
		v.cleanupTmpDir()
		return v, tea.Quit
	}
	return v, cmd
}

func (v *SkillsUpdateView) updateInstalling(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case skillsUpdateDoneMsg:
		v.successCount = msg.successCount
		v.updateErrors = msg.errors
		v.step = updateStepDone

		if v.successCount == 0 {
			return v, func() tea.Msg {
				return tui.ErrorMsg{Err: fmt.Errorf("failed to update skills for any tool")}
			}
		}

		v.doneMessage = v.summaryText()
		return v, nil
	}
	return v, nil
}

// ── View ────────────────────────────────────────────────────────────────────

func (v *SkillsUpdateView) View() string {
	var sb strings.Builder

	for _, line := range v.statusLines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if len(v.statusLines) > 0 {
		sb.WriteString("\n")
	}

	switch v.step {
	case updateStepSelectScope:
		if v.scopeForm != nil {
			sb.WriteString(v.scopeForm.View())
		}
	case updateStepSelectUpdates:
		if v.selectForm != nil {
			sb.WriteString(v.selectForm.View())
		}
	case updateStepDone:
		if v.doneMessage != "" {
			sb.WriteString(v.doneMessage)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// ── Commands ────────────────────────────────────────────────────────────────

func (v *SkillsUpdateView) loadStateCmd() tea.Cmd {
	return func() tea.Msg {
		return tui.LoadingDataMsg{
			LoadingMsgTmpl: "%s Loading saved selections...",
			Cmd: tea.Sequence(
				func() tea.Msg {
					loaded, err := skills.LoadOrRebuildState()
					if err != nil {
						return tui.ErrorMsg{Err: err}
					}

					return skillsUpdateStateLoadedMsg{
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

func (v *SkillsUpdateView) cloneRepoCmd() tea.Cmd {
	return func() tea.Msg {
		return tui.LoadingDataMsg{
			LoadingMsgTmpl: "%s Cloning skills repository...",
			Cmd: tea.Sequence(
				func() tea.Msg {
					prep, err := skills.PrepareInstall("")
					if err != nil {
						return tui.ErrorMsg{Err: err}
					}
					// Note: we don't call CleanupFn here; we'll clean up in updateInstallCmd
					return skillsUpdateClonedMsg{tmpDir: prep.TmpDir, remoteSkills: prep.Skills}
				},
				func() tea.Msg { return tui.DoneLoadingDataMsg{} },
			),
		}
	}
}

func (v *SkillsUpdateView) checkUpdatesCmd() tea.Cmd {
	// Create a scope-filtered copy so CheckForUpdates only sees skills
	// for the selected scope. The original v.state is preserved for saving.
	scope := v.scope
	scopedState := &skills.SkillsState{
		Tools: v.state.Tools,
	}
	for _, sk := range v.state.Skills {
		if sk.EffectiveScope() == scope {
			scopedState.Skills = append(scopedState.Skills, sk)
		}
	}

	tmpDir := v.tmpDir
	remoteSkills := v.remoteSkills
	force := v.force

	return func() tea.Msg {
		return tui.LoadingDataMsg{
			LoadingMsgTmpl: "%s Checking for updates...",
			Cmd: tea.Sequence(
				func() tea.Msg {
					result, err := skills.CheckForUpdates(scopedState, remoteSkills, tmpDir, force)
					if err != nil {
						return tui.ErrorMsg{Err: err}
					}

					return skillsUpdateCheckMsg{
						result:       result,
						remoteSkills: remoteSkills,
					}
				},
				func() tea.Msg { return tui.DoneLoadingDataMsg{} },
			),
		}
	}
}

func (v *SkillsUpdateView) updateInstallCmd() tea.Cmd {
	selectedTools := v.selectedTools
	tmpDir := v.tmpDir
	toUpdate := v.toUpdateNames
	outdated := v.outdated
	state := v.state
	remoteSkills := v.remoteSkills
	scope := v.scope
	repoRoot := v.repoRoot

	return func() tea.Msg {
		return tui.LoadingDataMsg{
			LoadingMsgTmpl: "%s Updating skills...",
			Cmd: tea.Sequence(
				func() tea.Msg {
					defer skills.CleanupTmpDir(tmpDir)

					// Filter outdated to only selected ones
					selectedOutdated := skills.FilterOutdatedByNames(outdated, toUpdate)

					result, err := skills.ExecuteUpdateWithScope(selectedTools, selectedOutdated, tmpDir, scope, repoRoot)
					if err != nil {
						return skillsUpdateDoneMsg{
							successCount: 0,
							errors:       []string{err.Error()},
						}
					}

					// Update state with new versions/hashes
					skills.UpdateStateAfterUpdate(state, selectedOutdated, remoteSkills, tmpDir, scope)

					return skillsUpdateDoneMsg{
						successCount: len(result.Tools) - len(result.Errors),
						errors:       result.Errors,
					}
				},
				func() tea.Msg { return tui.DoneLoadingDataMsg{} },
			),
		}
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (v *SkillsUpdateView) addStatus(format string, a ...any) {
	v.statusLines = append(v.statusLines, fmt.Sprintf(format, a...))
}

func (v *SkillsUpdateView) buildSelectedTools() {
	v.selectedTools = skills.IntersectToolsByState(v.detectedTools, v.state)
}

func (v *SkillsUpdateView) buildSelectForm() {
	// Pre-select all outdated. Use DirName as the value because
	// InstallSelectedSkills matches against directory names, not
	// frontmatter names.
	v.toUpdateNames = make([]string, len(v.outdated))
	for i, o := range v.outdated {
		v.toUpdateNames[i] = o.DirName
	}

	var options []huh.Option[string]
	for _, o := range v.outdated {
		options = append(options, huh.NewOption(fmt.Sprintf("%s  %s [%s]", o.Name, o.Label, scopeLabel(v.scope)), o.DirName))
	}

	v.selectForm = huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select skills to update").
				Options(options...).
				Value(&v.toUpdateNames),
		),
	)
}

func (v *SkillsUpdateView) saveState() {
	if v.state != nil {
		v.state.Touch()
		_ = v.state.Save()
	}
}

func (v *SkillsUpdateView) cleanupTmpDir() {
	if v.tmpDir != "" {
		skills.CleanupTmpDir(v.tmpDir)
		v.tmpDir = ""
	}
}

func (v *SkillsUpdateView) summaryText() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s Skills updated successfully!\n\n", iconCheck()))

	for _, e := range v.updateErrors {
		sb.WriteString(fmt.Sprintf("  %s %s\n", iconCross(), e))
	}

	sb.WriteString(fmt.Sprintf("%s Restart your AI coding tool to load the updated skills.", iconWarn()))
	return sb.String()
}

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

// ── Input ────────────────────────────────────────────────────────────────────

// SkillsInstallViewInput holds pre-populated input from CLI flags.
type SkillsInstallViewInput struct {
	// ToolFilter filters tools by name (from --tool flag).
	ToolFilter string
	// SkillFilter specifies which skills to install (from --skill flag).
	SkillFilter []string
	// Scope specifies where to install skills (from --scope flag).
	Scope skills.Scope
}

// ── Install step state machine ──────────────────────────────────────────────

type installStep int

const (
	installStepDetecting installStep = iota
	installStepSelectScope
	installStepSelectTools
	installStepCloning
	installStepSelectSkills
	installStepInstalling
	installStepDone
)

// ── Messages ────────────────────────────────────────────────────────────────

type skillsToolsDetectedMsg struct {
	tools []skills.Tool
}

type skillsRepoClonedMsg struct {
	tmpDir    string
	available []skills.SkillInfo
}

type skillsInstallDoneMsg struct {
	result *skills.InstallResult
	errors []string
}

// ── View ────────────────────────────────────────────────────────────────────

// SkillsInstallView implements the interactive skills install flow as a
// Bubble Tea model with an internal state machine.
type SkillsInstallView struct {
	step installStep

	// Input from CLI flags (for skipping steps)
	input SkillsInstallViewInput

	// Forms
	scopeForm *huh.Form
	toolForm  *huh.Form
	skillForm *huh.Form

	// Scope selection
	selectedScope string // "user" or "project"
	scope         skills.Scope
	repoRoot      string

	// Tool selection
	allTools          []skills.Tool
	selectedToolNames []string
	selectedTools     []skills.Tool

	// Skill selection
	available          []skills.SkillInfo
	selectedSkillNames []string

	// Clone
	tmpDir string

	// Install results
	result        *skills.InstallResult
	installErrors []string

	// Final result
	doneMessage string

	// Accumulated status lines for View()
	statusLines []string

	// Sizing
	width  int
	height int
}

func NewSkillsInstallView(input SkillsInstallViewInput) *SkillsInstallView {
	return &SkillsInstallView{
		step:  installStepDetecting,
		input: input,
	}
}

func (v *SkillsInstallView) Init() tea.Cmd {
	return v.detectToolsCmd()
}

// ── Update ──────────────────────────────────────────────────────────────────

func (v *SkillsInstallView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tui.StackSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		if v.scopeForm != nil {
			v.scopeForm = v.scopeForm.WithWidth(msg.Width).WithHeight(msg.Height)
		}
		if v.toolForm != nil {
			v.toolForm = v.toolForm.WithWidth(msg.Width).WithHeight(msg.Height)
		}
		if v.skillForm != nil {
			v.skillForm = v.skillForm.WithWidth(msg.Width).WithHeight(msg.Height)
		}
		return v, nil
	}

	switch v.step {
	case installStepDetecting:
		return v.updateDetecting(msg)
	case installStepSelectScope:
		return v.updateSelectScope(msg)
	case installStepSelectTools:
		return v.updateSelectTools(msg)
	case installStepCloning:
		return v.updateCloning(msg)
	case installStepSelectSkills:
		return v.updateSelectSkills(msg)
	case installStepInstalling:
		return v.updateInstalling(msg)
	case installStepDone:
		return v, nil
	}
	return v, nil
}

func (v *SkillsInstallView) updateDetecting(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case skillsToolsDetectedMsg:
		v.allTools = msg.tools

		if len(v.allTools) == 0 {
			return v, func() tea.Msg {
				return tui.ErrorMsg{Err: fmt.Errorf("no supported AI coding tools detected")}
			}
		}

		for _, t := range v.allTools {
			v.addStatus("  %s Found %s: %s", iconCheck(), renderstyle.Bold(t.Name), skills.ShortenPath(t.SkillsDir))
		}

		// Check if scope was pre-selected via CLI flag
		if v.input.Scope != "" {
			v.scope = v.input.Scope
			v.selectedScope = string(v.input.Scope)
			v.addStatus("  %s Using scope: %s", iconInfo(), v.input.Scope)

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

		// Show scope selection form
		v.scopeForm = buildScopeForm("Where do you want to install skills?", &v.selectedScope)
		v.step = installStepSelectScope
		return v, v.scopeForm.Init()
	}
	return v, nil
}

func (v *SkillsInstallView) updateSelectScope(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (v *SkillsInstallView) proceedAfterScopeSelection() (tea.Model, tea.Cmd) {
	// Check if we should skip tool selection
	if v.input.ToolFilter != "" {
		// Filter tools based on input
		v.selectedTools = skills.FilterTools(v.allTools, v.input.ToolFilter)
		if len(v.selectedTools) == 0 {
			return v, func() tea.Msg {
				return tui.ErrorMsg{Err: fmt.Errorf("no installed tool matching %q found", v.input.ToolFilter)}
			}
		}
		v.addStatus("  %s Using tool filter: %s", iconInfo(), v.input.ToolFilter)
		v.step = installStepCloning
		return v, v.cloneRepoCmd()
	}

	if len(v.allTools) == 1 {
		// Only one tool — skip the prompt.
		v.selectedToolNames = []string{v.allTools[0].Name}
		v.buildSelectedTools()
		v.step = installStepCloning
		return v, v.cloneRepoCmd()
	}

	v.buildToolForm()
	v.step = installStepSelectTools
	return v, v.toolForm.Init()
}

func (v *SkillsInstallView) updateSelectTools(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := v.toolForm.Update(msg)
	if m, ok := form.(*huh.Form); ok {
		v.toolForm = m
	}

	if v.toolForm.State == huh.StateCompleted {
		v.buildSelectedTools()
		if len(v.selectedTools) == 0 {
			return v, func() tea.Msg {
				return tui.ErrorMsg{Err: fmt.Errorf("no tools selected")}
			}
		}
		v.step = installStepCloning
		return v, v.cloneRepoCmd()
	}
	if v.toolForm.State == huh.StateAborted {
		return v, tea.Quit
	}
	return v, cmd
}

func (v *SkillsInstallView) updateCloning(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case skillsRepoClonedMsg:
		v.tmpDir = msg.tmpDir
		v.available = msg.available

		if len(v.available) == 0 {
			v.cleanupTmpDir()
			return v, func() tea.Msg {
				return tui.ErrorMsg{Err: fmt.Errorf("no skills found in the repository")}
			}
		}

		// Check if we should skip skill selection
		if len(v.input.SkillFilter) > 0 {
			// Resolve filter to directory names
			v.selectedSkillNames, _ = skills.ResolveSkillNames(v.available, v.input.SkillFilter)
			if len(v.selectedSkillNames) == 0 {
				v.cleanupTmpDir()
				return v, func() tea.Msg {
					return tui.ErrorMsg{Err: fmt.Errorf("no skills matching filter found")}
				}
			}
			v.addStatus("  %s Using skill filter: %s", iconInfo(), strings.Join(v.input.SkillFilter, ", "))
			v.step = installStepInstalling
			return v, v.installCmd()
		}

		v.buildSkillForm()
		v.step = installStepSelectSkills
		return v, v.skillForm.Init()
	}
	return v, nil
}

func (v *SkillsInstallView) updateSelectSkills(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := v.skillForm.Update(msg)
	if m, ok := form.(*huh.Form); ok {
		v.skillForm = m
	}

	if v.skillForm.State == huh.StateCompleted {
		if len(v.selectedSkillNames) == 0 {
			v.cleanupTmpDir()
			return v, func() tea.Msg {
				return tui.ErrorMsg{Err: fmt.Errorf("no skills selected")}
			}
		}
		v.step = installStepInstalling
		return v, v.installCmd()
	}
	if v.skillForm.State == huh.StateAborted {
		v.cleanupTmpDir()
		return v, tea.Quit
	}
	return v, cmd
}

func (v *SkillsInstallView) updateInstalling(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case skillsInstallDoneMsg:
		v.result = msg.result
		v.installErrors = msg.errors

		v.step = installStepDone

		if v.result == nil {
			return v, func() tea.Msg {
				return tui.ErrorMsg{Err: fmt.Errorf("failed to install skills to any tool")}
			}
		}

		v.doneMessage = v.summaryText()
		return v, nil
	}
	return v, nil
}

// ── View ────────────────────────────────────────────────────────────────────

func (v *SkillsInstallView) View() string {
	var sb strings.Builder

	// Render accumulated status lines.
	for _, line := range v.statusLines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if len(v.statusLines) > 0 {
		sb.WriteString("\n")
	}

	switch v.step {
	case installStepSelectScope:
		if v.scopeForm != nil {
			sb.WriteString(v.scopeForm.View())
		}
	case installStepSelectTools:
		if v.toolForm != nil {
			sb.WriteString(v.toolForm.View())
		}
	case installStepSelectSkills:
		if v.skillForm != nil {
			sb.WriteString(v.skillForm.View())
		}
	case installStepDone:
		if v.doneMessage != "" {
			sb.WriteString(v.doneMessage)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// ── Commands ────────────────────────────────────────────────────────────────

func (v *SkillsInstallView) detectToolsCmd() tea.Cmd {
	return func() tea.Msg {
		return tui.LoadingDataMsg{
			LoadingMsgTmpl: "%s Detecting tools...",
			Cmd: tea.Sequence(
				func() tea.Msg {
					tools, err := skills.DetectTools()
					if err != nil {
						return tui.ErrorMsg{Err: fmt.Errorf("failed to detect tools: %w", err)}
					}
					return skillsToolsDetectedMsg{tools: tools}
				},
				func() tea.Msg { return tui.DoneLoadingDataMsg{} },
			),
		}
	}
}

func (v *SkillsInstallView) cloneRepoCmd() tea.Cmd {
	return func() tea.Msg {
		return tui.LoadingDataMsg{
			LoadingMsgTmpl: "%s Cloning skills repository...",
			Cmd: tea.Sequence(
				func() tea.Msg {
					prep, err := skills.PrepareInstall("")
					if err != nil {
						return tui.ErrorMsg{Err: err}
					}
					// Note: we don't call CleanupFn here; we'll clean up in installCmd
					return skillsRepoClonedMsg{tmpDir: prep.TmpDir, available: prep.Skills}
				},
				func() tea.Msg { return tui.DoneLoadingDataMsg{} },
			),
		}
	}
}

func (v *SkillsInstallView) installCmd() tea.Cmd {
	selectedTools := v.selectedTools
	tmpDir := v.tmpDir
	selectedSkillNames := v.selectedSkillNames
	scope := v.scope
	repoRoot := v.repoRoot

	return func() tea.Msg {
		return tui.LoadingDataMsg{
			LoadingMsgTmpl: "%s Installing skills...",
			Cmd: tea.Sequence(
				func() tea.Msg {
					defer func() {
						// Cleanup temp dir after install
						skills.CleanupTmpDir(tmpDir)
					}()

					result, err := skills.ExecuteInstallWithScope(selectedTools, selectedSkillNames, tmpDir, scope, repoRoot)
					if err != nil {
						return skillsInstallDoneMsg{
							result: nil,
							errors: []string{err.Error()},
						}
					}

					return skillsInstallDoneMsg{
						result: result,
						errors: nil,
					}
				},
				func() tea.Msg { return tui.DoneLoadingDataMsg{} },
			),
		}
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (v *SkillsInstallView) addStatus(format string, a ...any) {
	v.statusLines = append(v.statusLines, fmt.Sprintf(format, a...))
}

func (v *SkillsInstallView) buildToolForm() {
	toolNames := make([]string, len(v.allTools))
	for i, t := range v.allTools {
		toolNames[i] = t.Name
	}

	// Pre-select all tools.
	v.selectedToolNames = make([]string, len(toolNames))
	copy(v.selectedToolNames, toolNames)

	var options []huh.Option[string]
	for _, name := range toolNames {
		options = append(options, huh.NewOption(name, name))
	}

	v.toolForm = huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select tools to install skills to").
				Options(options...).
				Value(&v.selectedToolNames),
		),
	)
}

func (v *SkillsInstallView) buildSkillForm() {
	// Pre-select all skills. Use DirName as the value because
	// InstallSelectedSkills matches against directory names, not
	// frontmatter names.
	v.selectedSkillNames = make([]string, len(v.available))
	for i, s := range v.available {
		v.selectedSkillNames[i] = s.DirName
	}

	var options []huh.Option[string]
	for _, s := range v.available {
		label := s.Name
		if s.Description != "" {
			label = fmt.Sprintf("%s — %s", s.Name, skillsFirstSentence(s.Description))
		}
		options = append(options, huh.NewOption(label, s.DirName))
	}

	v.skillForm = huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select skills to install").
				Options(options...).
				Value(&v.selectedSkillNames),
		),
	)
}

func (v *SkillsInstallView) buildSelectedTools() {
	v.selectedTools = skills.FilterToolsByNames(v.allTools, v.selectedToolNames)
}

func (v *SkillsInstallView) cleanupTmpDir() {
	if v.tmpDir != "" {
		skills.CleanupTmpDir(v.tmpDir)
		v.tmpDir = ""
	}
}

func (v *SkillsInstallView) summaryText() string {
	var sb strings.Builder
	check := iconCheck()
	warn := iconWarn()

	scopeLabel := "user"
	if v.scope == skills.ScopeProject {
		scopeLabel = "project"
	}
	sb.WriteString(fmt.Sprintf("%s Skills installed successfully (%s scope)!\n\n", check, scopeLabel))

	dim := lipgloss.NewStyle().Foreground(renderstyle.ColorDeprioritized)

	if v.result != nil && len(v.result.Skills) > 0 {
		sb.WriteString("Installed skills:\n")
		for _, s := range v.result.Skills {
			sb.WriteString(fmt.Sprintf("  • %s:", renderstyle.Bold(s.Name)))
			desc := skillsFirstSentence(s.Description)
			if desc != "" {
				sb.WriteString(fmt.Sprintf("\n    %s", dim.Render(desc)))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	for _, e := range v.installErrors {
		sb.WriteString(fmt.Sprintf("  %s %s\n", iconCross(), e))
	}

	sb.WriteString(fmt.Sprintf("%s Restart your AI coding tool to load the new skills.", warn))
	return sb.String()
}

func skillsFirstSentence(s string) string {
	if i := strings.Index(s, ". "); i >= 0 {
		return s[:i+1]
	}
	return s
}

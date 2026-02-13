package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/render-oss/cli/pkg/skills"
	renderstyle "github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/tui"
)

// ── List step state machine ─────────────────────────────────────────────────

type listStep int

const (
	listStepLoading listStep = iota
	listStepDone
)

// ── Messages ────────────────────────────────────────────────────────────────

type skillsListLoadedMsg struct {
	state         *skills.SkillsState
	detectedTools []skills.Tool
	warnings      []string
}

// ── View ────────────────────────────────────────────────────────────────────

// SkillsListView displays installed skills and detected tools.
type SkillsListView struct {
	step listStep

	// Scope filter (optional)
	scopeFilter skills.Scope

	// Loaded data
	state         *skills.SkillsState
	detectedTools []skills.Tool

	// Rendered output
	output string

	// Sizing
	width  int
	height int
}

func NewSkillsListView(scopeFilter skills.Scope) *SkillsListView {
	return &SkillsListView{
		step:        listStepLoading,
		scopeFilter: scopeFilter,
	}
}

func (v *SkillsListView) Init() tea.Cmd {
	return v.loadCmd()
}

// ── Update ──────────────────────────────────────────────────────────────────

func (v *SkillsListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tui.StackSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return v, nil
	}

	switch v.step {
	case listStepLoading:
		return v.updateLoading(msg)
	case listStepDone:
		return v, nil
	}
	return v, nil
}

func (v *SkillsListView) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case skillsListLoadedMsg:
		v.state = msg.state
		v.detectedTools = msg.detectedTools
		v.output = v.renderOutput(msg.warnings)
		v.step = listStepDone
		return v, nil
	}
	return v, nil
}

// ── View ────────────────────────────────────────────────────────────────────

func (v *SkillsListView) View() string {
	if v.step == listStepDone {
		return v.output
	}
	return ""
}

// ── Commands ────────────────────────────────────────────────────────────────

func (v *SkillsListView) loadCmd() tea.Cmd {
	return func() tea.Msg {
		return tui.LoadingDataMsg{
			LoadingMsgTmpl: "%s Loading skills...",
			Cmd: tea.Sequence(
				func() tea.Msg {
					loaded, err := skills.LoadOrRebuildState()
					if err != nil {
						return tui.ErrorMsg{Err: err}
					}

					return skillsListLoadedMsg{
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

// ── Rendering ───────────────────────────────────────────────────────────────

func (v *SkillsListView) renderOutput(warnings []string) string {
	var sb strings.Builder

	dim := lipgloss.NewStyle().Foreground(renderstyle.ColorDeprioritized)

	for _, w := range warnings {
		sb.WriteString(fmt.Sprintf("  %s %s\n", iconWarn(), w))
	}

	// No tools detected and no state.
	if len(v.detectedTools) == 0 && (!v.state.HasSelections() || len(v.state.Skills) == 0) {
		sb.WriteString(fmt.Sprintf("%s No supported AI coding tools detected\n", iconCross()))
		return sb.String()
	}

	// Filter skills by scope if specified
	filteredSkills := v.state.Skills
	if v.scopeFilter != "" {
		filteredSkills = nil
		for _, sk := range v.state.Skills {
			if sk.EffectiveScope() == v.scopeFilter {
				filteredSkills = append(filteredSkills, sk)
			}
		}
	}

	// No skills installed.
	if len(filteredSkills) == 0 {
		if v.scopeFilter != "" {
			sb.WriteString(fmt.Sprintf("%s No Render skills installed at %s scope\n\n", iconInfo(), v.scopeFilter))
		} else {
			sb.WriteString(fmt.Sprintf("%s No Render skills installed\n\n", iconInfo()))
		}
		sb.WriteString(fmt.Sprintf("  Run %s to get started.\n", renderstyle.Bold("render skills install")))
		return sb.String()
	}

	// ── Skills ──────────────────────────────────────────────────────────
	if v.scopeFilter != "" {
		sb.WriteString(fmt.Sprintf("Installed skills (%s scope):\n\n", v.scopeFilter))
	} else {
		sb.WriteString("Installed skills:\n\n")
	}
	for _, sk := range filteredSkills {
		version := sk.Version
		if version == "" || version == "unknown" {
			version = "no version"
		}
		sl := scopeLabel(sk.EffectiveScope())
		sb.WriteString(fmt.Sprintf("  %s %s  %s %s\n", iconCheck(), renderstyle.Bold(sk.Name), dim.Render(version), dim.Render("["+sl+"]")))
	}
	sb.WriteString("\n")

	// ── Tools ───────────────────────────────────────────────────────────
	detectedMap := make(map[string]skills.Tool, len(v.detectedTools))
	for _, t := range v.detectedTools {
		detectedMap[t.Name] = t
	}

	sb.WriteString("Tools:\n\n")
	for _, name := range v.state.Tools {
		if t, ok := detectedMap[name]; ok {
			sb.WriteString(fmt.Sprintf("  %s %s  %s\n", iconCheck(), renderstyle.Bold(t.Name), dim.Render(skills.ShortenPath(t.SkillsDir))))
		} else {
			sb.WriteString(fmt.Sprintf("  %s %s  %s\n", iconCross(), renderstyle.Bold(name), dim.Render("(not detected)")))
		}
	}
	sb.WriteString("\n")

	if v.state.InstalledAt != "" {
		sb.WriteString(fmt.Sprintf("Last updated: %s\n", dim.Render(v.state.InstalledAt)))
	}

	return sb.String()
}

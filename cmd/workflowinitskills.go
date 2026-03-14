package cmd

import (
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/skills"
	renderstyle "github.com/render-oss/cli/pkg/style"
)

func promptSkillInstall(cmd *cobra.Command) {
	ok := lipgloss.NewStyle().Foreground(renderstyle.ColorOK)
	dim := lipgloss.NewStyle().Foreground(renderstyle.ColorDeprioritized)

	// Skip if render-workflows is already installed
	if state, err := skills.LoadState(); err == nil {
		for _, s := range state.Skills {
			if s.Name == "render-workflows" {
				return
			}
		}
	}

	detectedTools, err := skills.DetectTools()
	if err != nil || len(detectedTools) == 0 {
		return
	}

	command.Println(cmd, "")

	var installChoice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("(Optional) Install Workflows agent skill for your AI coding assistant?").
				Description("This is an optional enhancement. It helps Cursor, Copilot, Claude Code, etc. understand the Render Workflows SDK when helping you write tasks.").
				Options(
					huh.NewOption("Yes", "yes"),
					huh.NewOption("No", "no"),
				).
				Value(&installChoice),
		),
	)
	if err := form.Run(); err != nil || installChoice != "yes" {
		cmdStyle := lipgloss.NewStyle().Foreground(renderstyle.ColorInfo).Bold(true)
		command.Println(cmd, "  %s Agent skill: no. You can install Render agent skills later by running %s.",
			ok.Render("✓"),
			cmdStyle.Render("render skills"))
		return
	}

	// If multiple tools, prompt which ones to install to
	selectedTools := promptToolSelection(detectedTools)
	if len(selectedTools) == 0 {
		cmdStyle := lipgloss.NewStyle().Foreground(renderstyle.ColorInfo).Bold(true)
		command.Println(cmd, "  %s Agent skill: no tools selected. You can install Render agent skills later by running %s.",
			ok.Render("✓"),
			cmdStyle.Render("render skills"))
		return
	}

	fail := lipgloss.NewStyle().Foreground(renderstyle.ColorError)

	result, err := skills.Install(skills.InstallInput{
		PreSelectedTools: selectedTools,
		SkillFilter:      []string{"render-workflows"},
	})
	skillStyle := lipgloss.NewStyle().Foreground(renderstyle.ColorInfo)

	if err != nil {
		command.Println(cmd, "  %s Agent skill failed to install (%s)", fail.Render("✗"), err)
	} else {
		toolNames := strings.Join(skills.ToolNames(result.Tools), ", ")
		command.Println(cmd, "  %s Agent skill installed: %s %s %s",
			ok.Render("✓"),
			skillStyle.Render("render-workflows"),
			dim.Render("→"),
			toolNames,
		)
	}

}

// promptToolSelection asks the user which detected tools to install to.
// If only one tool is detected, it is returned directly.
func promptToolSelection(detectedTools []skills.Tool) []skills.Tool {
	if len(detectedTools) == 1 {
		return detectedTools
	}

	var selectedNames []string
	toolOptions := make([]huh.Option[string], len(detectedTools))
	for i, t := range detectedTools {
		toolOptions[i] = huh.NewOption(t.Name, t.Name)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which tools should we install the skill to?").
				Description("Select one or more with space, then press enter.").
				Options(toolOptions...).
				Value(&selectedNames),
		),
	)
	if err := form.Run(); err != nil {
		return nil
	}

	return skills.FilterToolsByNames(detectedTools, selectedNames)
}

func installSkillNonInteractive(cmd *cobra.Command) {
	dim := lipgloss.NewStyle().Foreground(renderstyle.ColorDeprioritized)
	ok := lipgloss.NewStyle().Foreground(renderstyle.ColorOK)
	fail := lipgloss.NewStyle().Foreground(renderstyle.ColorError)
	skillStyle := lipgloss.NewStyle().Foreground(renderstyle.ColorInfo)

	result, err := skills.Install(skills.InstallInput{
		SkillFilter: []string{"render-workflows"},
	})
	if err != nil {
		command.Println(cmd, "%s Could not install agent skill: %s", fail.Render("✗"), err)
		return
	}

	toolNames := strings.Join(skills.ToolNames(result.Tools), ", ")
	command.Println(cmd, "%s Installed %s skill %s %s",
		ok.Render("✓"),
		skillStyle.Render("render-workflows"),
		dim.Render("→"),
		toolNames,
	)
}

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/skills"
	renderstyle "github.com/render-oss/cli/pkg/style"
)

var skillsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Render skills to AI coding tools",
	Long: `Install Render agent skills from https://github.com/render-oss/skills to
detected AI coding tools.

Supported tools: Claude Code, Codex, OpenCode, Cursor.

Skills are installed to each tool's skills directory (e.g. ~/.cursor/skills).
Only tools that are already set up on your system are detected.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSkillsInstall(cmd)
	},
}

func init() {
	skillsCmd.AddCommand(skillsInstallCmd)
	skillsInstallCmd.Flags().String("tool", "", "install to a specific tool only (claude, codex, opencode, cursor)")
	skillsInstallCmd.Flags().Bool("dry-run", false, "show what would be installed without making changes")
}

func runSkillsInstall(cmd *cobra.Command) error {
	toolFilter, _ := cmd.Flags().GetString("tool")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	okStyle := lipgloss.NewStyle().Foreground(renderstyle.ColorOK)
	infoStyle := lipgloss.NewStyle().Foreground(renderstyle.ColorInfo)
	warnStyle := lipgloss.NewStyle().Foreground(renderstyle.ColorWarning)
	errStyle := lipgloss.NewStyle().Foreground(renderstyle.ColorError)

	check := okStyle.Render("✓")
	info := infoStyle.Render("ℹ")
	warn := warnStyle.Render("⚠")
	cross := errStyle.Render("✗")

	// Detect tools.
	command.Println(cmd, "%s Detecting installed AI coding tools...", info)
	command.Println(cmd, "")

	tools, err := skills.DetectTools()
	if err != nil {
		return fmt.Errorf("failed to detect tools: %w", err)
	}

	if toolFilter != "" {
		tools = skills.FilterTools(tools, toolFilter)
		if len(tools) == 0 {
			return fmt.Errorf("no installed tool matching %q found", toolFilter)
		}
	}

	if len(tools) == 0 {
		command.Println(cmd, "%s No supported AI coding tools detected", cross)
		command.Println(cmd, "  Supported: Claude Code, Codex, OpenCode, Cursor")
		return fmt.Errorf("no tools detected")
	}

	for _, t := range tools {
		command.Println(cmd, "  %s Found %s: %s", check, t.Name, skills.ShortenPath(t.SkillsDir))
	}
	command.Println(cmd, "")

	// Clone the skills repo.
	command.Println(cmd, "%s Cloning skills repository...", info)

	tmpDir, err := os.MkdirTemp("", "render-skills-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := skills.CloneSkillsRepo(tmpDir); err != nil {
		return err
	}
	command.Println(cmd, "%s Repository cloned", check)
	command.Println(cmd, "")

	if dryRun {
		available := skills.ReadSkillsFromRepo(tmpDir)
		command.Println(cmd, "%s Dry run: would install %d skill(s) to %d tool(s)", info, len(available), len(tools))
		command.Println(cmd, "")
		printSkillList(cmd, available)
		return nil
	}

	// Install to each tool.
	command.Println(cmd, "%s Installing skills...", info)
	command.Println(cmd, "")

	successCount := 0
	var lastInstalled []skills.SkillInfo
	for _, t := range tools {
		installed, err := skills.InstallSkills(t.SkillsDir, tmpDir)
		if err != nil {
			command.Println(cmd, "  %s %s: %s", cross, t.Name, err)
			continue
		}
		command.Println(cmd, "  %s Installed %d skill(s) to %s", check, len(installed), skills.ShortenPath(t.SkillsDir))
		lastInstalled = installed
		successCount++
	}

	command.Println(cmd, "")

	if successCount == 0 {
		return fmt.Errorf("failed to install skills to any tool")
	}

	// Summary.
	command.Println(cmd, "%s Skills installed successfully!", check)
	command.Println(cmd, "")
	printSkillList(cmd, lastInstalled)
	command.Println(cmd, "%s Restart your AI coding tool to load the new skills.", warn)

	return nil
}

func printSkillList(cmd *cobra.Command, installed []skills.SkillInfo) {
	dimStyle := lipgloss.NewStyle().Foreground(renderstyle.ColorDeprioritized)

	command.Println(cmd, "Available skills:")
	for _, s := range installed {
		desc := firstSentence(s.Description)
		command.Println(cmd, "  • %s:", renderstyle.Bold(s.Name))
		if desc != "" {
			command.Println(cmd, "    %s", dimStyle.Render(desc))
		}
	}
	command.Println(cmd, "")
}

// firstSentence returns the text up to and including the first period.
func firstSentence(s string) string {
	if i := strings.Index(s, ". "); i >= 0 {
		return s[:i+1]
	}
	return s
}

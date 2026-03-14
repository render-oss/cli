package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	renderstyle "github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/workflows/scaffold"
)

type WorkflowInitInput struct {
	Language          string `cli:"language"`
	Template          string `cli:"template"`
	Dir               string `cli:"dir"`
	InstallDeps       bool   `cli:"install-deps"`
	Git               bool   `cli:"git"`
	InstallAgentSkill bool   `cli:"install-agent-skill"`
}

const defaultDir = "workflows-demo"

var workflowInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a new workflows project",
	Long: `Scaffold a new workflows project with example tasks.

Creates a working example project with task definitions, dependencies,
and a README with instructions for local development and Client SDK
integration.

In interactive mode you'll be prompted to select a language, template,
output directory, and optional features. Use --confirm to skip all
prompts and accept defaults, or pass individual flags to skip specific
prompts.

Examples:
  render workflows init
  render workflows init --confirm --language python
  render workflows init --language python --dir my-project --install-deps --git
  render workflows init --language node --dir my-project`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("output") {
			output, _ := cmd.Flags().GetString("output")
			if output == "json" || output == "yaml" {
				return fmt.Errorf("--output %s is not supported for this command", output)
			}
		}

		var input WorkflowInitInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		runner := &WorkflowInitRunner{
			deps:        &defaultInitDeps{},
			interactive: command.IsInteractive(cmd.Context()),
			cmd:         cmd,
		}
		return runner.Run(cmd.Context(), input)
	},
}

func pluralize(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// shellWrap wraps a shell command to fit within width characters, inserting "
// \" continuation characters at word boundaries. Continuation lines are
// indented by contIndent spaces to align with the first line's content.
// I know it doesn't feel right that we're rolling our own word wrapping logic,
// but I wanted special handling for wrapped shell commands.
func shellWrap(cmd string, width int, contIndent int) string {
	if len(cmd) <= width {
		return cmd
	}

	continuation := " \\\n" + strings.Repeat(" ", contIndent+2) // align past "$ "
	var b strings.Builder
	lineLen := 0

	words := strings.Fields(cmd)
	for i, word := range words {
		wordLen := len(word)
		if i > 0 {
			wordLen++ // account for space before word
		}

		if i > 0 && lineLen+wordLen > width {
			b.WriteString(continuation)
			lineLen = contIndent + 2 // continuation indent
			b.WriteString(word)
			lineLen += len(word)
		} else {
			if i > 0 {
				b.WriteByte(' ')
				lineLen++
			}
			b.WriteString(word)
			lineLen += len(word)
		}
	}

	return b.String()
}

func formatNextSteps(result *scaffold.Result, relDir string) string {
	dim := lipgloss.NewStyle().Foreground(renderstyle.ColorDeprioritized)
	info := lipgloss.NewStyle().Foreground(renderstyle.ColorInfo)

	const contentIndent = 3 // indent for command/hint lines below step label
	maxWidth := 80
	termWidth := 80
	if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
		termWidth = w
	}
	contentWidth := min(termWidth, maxWidth)

	labelWidth := contentWidth - 3 // account for "N. " prefix

	cmdStyle := lipgloss.NewStyle().
		Foreground(renderstyle.ColorInfo).
		Bold(true)

	hintStyle := lipgloss.NewStyle().
		Foreground(renderstyle.ColorDeprioritized).
		Width(contentWidth).
		PaddingLeft(contentIndent + 2) // +2 for outer indent

	cmdWidth := contentWidth - contentIndent - 2 - 2 // account for outer indent + inner indent + "$ "

	var b strings.Builder

	const outerIndent = "  "

	fmt.Fprintf(&b, "\n%s%s\n", outerIndent, renderstyle.Bold("Next steps"))
	fmt.Fprintf(&b, "%s%s\n", outerIndent, dim.Render("─────────────────────────────────────────────"))
	fmt.Fprintf(&b, "\n")

	indent := outerIndent + strings.Repeat(" ", contentIndent)

	formatStep := func(n int, label string, cmdText string, hint string) {
		styledLabel := lipgloss.NewStyle().Bold(true).Width(labelWidth).Render(label)
		fmt.Fprintf(&b, "%s%s %s\n", outerIndent, info.Render(fmt.Sprintf("%d.", n)), styledLabel)
		if cmdText != "" {
			wrapped := shellWrap(cmdText, cmdWidth, contentIndent)
			fmt.Fprintf(&b, "%s%s %s\n", indent, dim.Render("$"), cmdStyle.Render(wrapped))
		}
		if hint != "" {
			fmt.Fprintf(&b, "%s\n", strings.TrimRight(hintStyle.Render(hint), " \n"))
		}
	}

	// Two interpolation modes:
	// - plain: for command lines where shellWrap counts raw characters
	// - styled: for labels/hints where lipgloss Width() handles ANSI correctly
	setupCmd := scaffold.SetupCommand(result.Language)

	// Plain/styled pairs for each placeholder
	type placeholder struct{ plain, styled string }
	vars := map[string]placeholder{
		"{{buildCommand}}":       {result.BuildCommand, cmdStyle.Render(result.BuildCommand)},
		"{{startCommand}}":       {result.StartCommand, cmdStyle.Render(result.StartCommand)},
		"{{renderBuildCommand}}": {result.RenderBuildCommand, cmdStyle.Render(result.RenderBuildCommand)},
		"{{renderStartCommand}}": {result.RenderStartCommand, cmdStyle.Render(result.RenderStartCommand)},
		"{{setupCommand}}":       {setupCmd, cmdStyle.Render(setupCmd)},
		"{{dir}}":                {relDir, relDir},
	}

	interpolatePlain := func(s string) string {
		for key, v := range vars {
			s = strings.ReplaceAll(s, key, v.plain)
		}
		return s
	}

	interpolateStyled := func(s string) string {
		for key, v := range vars {
			s = strings.ReplaceAll(s, key, v.styled)
		}
		return s
	}

	for i, step := range result.NextSteps {
		if i > 0 {
			fmt.Fprintf(&b, "\n")
		}
		formatStep(i+1,
			interpolateStyled(step.Label),
			interpolatePlain(step.Command),
			interpolateStyled(step.Hint),
		)
	}

	// Call to action
	fmt.Fprintf(&b, "%s%s\n", outerIndent, dim.Render("─────────────────────────────────────────────"))
	fmt.Fprintf(&b, "%s%s\n", outerIndent, dim.Render("Visit our docs to learn more: https://render.com/docs/workflows"))

	return b.String()
}

func init() {
	workflowInitCmd.Flags().String("language", "", "Language for the workflows project (python, node)")
	workflowInitCmd.Flags().String("template", "", "Template to scaffold (defaults to the repo's default template)")
	workflowInitCmd.Flags().String("dir", "", "Output directory (default: workflows-demo)")
	workflowInitCmd.Flags().Bool("install-deps", false, "Install dependencies after scaffolding")
	workflowInitCmd.Flags().Bool("git", false, "Initialize a git repository")
	workflowInitCmd.Flags().Bool("install-agent-skill", false, "Install the Workflows agent skill for detected AI coding tools")

	// --output doesn't apply to this command (output is always text/interactive)
	workflowInitCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		cmd.InheritedFlags().MarkHidden("output")
		cmd.Parent().HelpFunc()(cmd, args)
	})

	WorkflowsCmd.AddCommand(workflowInitCmd)
}

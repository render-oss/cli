package cmd

import (
	"fmt"
	"os"
	"path/filepath"
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

const defaultDir = "./workflows-demo"

var workflowInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a new Render Workflows project",
	Long: `Scaffold a new Render Workflows project with example tasks.

Creates a working example project with task definitions, dependencies, and a README with instructions for local development and Client SDK integration.

In interactive mode you'll be prompted to select a language, template, output directory, and optional features. Use --confirm to skip all prompts and accept defaults, or pass individual flags to skip specific prompts.

With --confirm or non-interactive output (-o text/json/yaml), dependencies are installed and Git is initialized by default. Pass --install-deps=false or --git=false to opt out.`,
	Example: `  # Scaffold with default settings
  render workflows init

  # Skip prompts and use Python
  render workflows init --confirm --language python

  # Skip prompts and disable Git initialization
  render workflows init --confirm --language python --git=false

  # Customize output directory and enable optional features
  render workflows init --language python --dir my-project --install-deps --git

  # Use Node.js with a custom directory
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

func formatNextSteps(result *scaffold.Result, relDir string, gitInitialized bool) string {
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

	placeholderStyle := lipgloss.NewStyle().
		Foreground(renderstyle.ColorWarning).
		Bold(true)

	formatMultilineStep := func(n int, label string, cmdLines []string, hint string) {
		styledLabel := lipgloss.NewStyle().Bold(true).Width(labelWidth).Render(label)
		fmt.Fprintf(&b, "%s%s %s\n", outerIndent, info.Render(fmt.Sprintf("%d.", n)), styledLabel)
		if len(cmdLines) > 0 {
			fmt.Fprintf(&b, "%s%s %s\n", indent, dim.Render("$"), cmdStyle.Render(cmdLines[0]))
			contIndent := indent + "  "
			for _, line := range cmdLines[1:] {
				// Highlight placeholders in a different color
				if idx := strings.Index(line, "<"); idx >= 0 {
					if end := strings.Index(line[idx:], ">"); end >= 0 {
						placeholder := line[idx : idx+end+1]
						styled := cmdStyle.Render(line[:idx]) +
							placeholderStyle.Render(placeholder) +
							cmdStyle.Render(line[idx+end+1:])
						fmt.Fprintf(&b, "%s%s\n", contIndent, styled)
						continue
					}
				}
				fmt.Fprintf(&b, "%s%s\n", contIndent, cmdStyle.Render(line))
			}
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

	// Filter out the legacy template-defined deploy step. Templates still
	// include it for compatibility with older CLI versions (up to 2.15.1), but
	// now the CLI version renders its own deploy step below with a
	// pre-populated `render workflows create` command.
	templateSteps := make([]scaffold.NextStep, 0, len(result.NextSteps))
	for _, step := range result.NextSteps {
		if step.Label == "Deploy your workflow service to Render" {
			continue
		}
		templateSteps = append(templateSteps, step)
	}

	for i, step := range templateSteps {
		if i > 0 {
			fmt.Fprintf(&b, "\n")
		}
		formatStep(i+1,
			interpolateStyled(step.Label),
			interpolatePlain(step.Command),
			interpolateStyled(step.Hint),
		)
	}

	// Git & deploy steps: always appended for all templates
	if len(templateSteps) > 0 {
		fmt.Fprintf(&b, "\n")
	}

	formatStep(len(templateSteps)+1, "Push your project repo to your Git provider", "", "Render can pull from GitHub, GitLab, or Bitbucket.")
	fmt.Fprintf(&b, "\n")

	deploy := buildDeployStep(relDir, string(result.Language), result.RenderBuildCommand, result.RenderStartCommand, gitInitialized)
	formatMultilineStep(len(templateSteps)+2, deploy.Label, deploy.CmdLines, deploy.Hint)

	// Call to action
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "%s%s\n", outerIndent, dim.Render("─────────────────────────────────────────────"))
	fmt.Fprintf(&b, "%s%s\n", outerIndent, dim.Render("Visit our docs to learn more: https://render.com/docs/workflows"))

	return b.String()
}

// deployStep bundles the label, command lines, and hint for the final
// "Deploy your workflow on Render" step, so its content can be tested
// without going through the styling/rendering path.
type deployStep struct {
	Label    string
	CmdLines []string
	Hint     string
}

// buildDeployStep assembles the deploy step.
func buildDeployStep(relDir, runtime, buildCmd, runCmd string, gitInitialized bool) deployStep {
	name := filepath.Base(relDir)
	hint := "Replace <your-repo-url> with your Git repository URL."
	if gitInitialized {
		hint = fmt.Sprintf("Run this from the %s directory after pushing to your Git provider.", relDir)
	}
	return deployStep{
		Label:    "Deploy your workflow on Render:",
		CmdLines: buildDeployCommand(name, runtime, buildCmd, runCmd, gitInitialized),
		Hint:     hint,
	}
}

// buildDeployCommand returns the lines of a multi-line `render workflows create`
// command pre-populated with values from the scaffolded project. When
// localRepo is true, the command emits `--repo .` to infer the URL from the
// local git remote instead of a placeholder.
func buildDeployCommand(name, runtime, buildCmd, runCmd string, localRepo bool) []string {
	repoArg := "  --repo <your-repo-url>"
	if localRepo {
		repoArg = "  --repo ."
	}
	return []string{
		"render workflows create \\",
		fmt.Sprintf("  --name %q \\", name),
		fmt.Sprintf("  --runtime %s \\", runtime),
		fmt.Sprintf("  --build-command %q \\", buildCmd),
		fmt.Sprintf("  --run-command %q \\", runCmd),
		repoArg,
	}
}

func init() {
	workflowInitCmd.Flags().String("language", "", "Language for the Render Workflows project (python, node)")
	workflowInitCmd.Flags().String("template", "", "Template to scaffold (defaults to the repo's default template)")
	workflowInitCmd.Flags().String("dir", "", "Output directory (default: ./workflows-demo)")
	workflowInitCmd.Flags().Bool("install-deps", false, "Install dependencies after scaffolding (default true with --confirm)")
	workflowInitCmd.Flags().Bool("git", false, "Initialize a Git repository (default true with --confirm)")
	workflowInitCmd.Flags().Bool("install-agent-skill", false, "Install the Workflows agent skill for detected AI coding tools")

	// --output doesn't apply to this command (output is always text/interactive)
	workflowInitCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		cmd.InheritedFlags().MarkHidden("output")
		cmd.Parent().HelpFunc()(cmd, args)
	})

	WorkflowsCmd.AddCommand(workflowInitCmd)
}

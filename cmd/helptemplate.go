package cmd

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/style"
	"github.com/spf13/cobra"
)

var wrapTextTokenPattern = regexp.MustCompile(`\s+|\S+`)

const maxHelpTextWidth = 80

// cliVersion returns a styled version string for the help template
func cliVersion() string {
	return style.SubtleText.Faint(true).Render("Render CLI v" + cfg.Version)
}

// groupHeaderText returns styled text for group headers
func groupHeaderText(text string) string {
	return style.GroupHeader.Render(text)
}

// getUsageArgs extracts just the arguments from the Use field (everything after the command name)
// Example: "cancel <serviceID> <deployID>" returns " <serviceID> <deployID>"
// Example: "services" returns ""
func getUsageArgs(use string) string {
	fields := strings.Fields(use)
	if len(fields) <= 1 {
		return ""
	}
	// Everything after the first field (command name) is arguments
	return " " + strings.Join(fields[1:], " ")
}

// formatExamples formats examples with gray comments
func formatExamples(text string) string {
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			// Comment line - apply gray styling
			result = append(result, style.SubtleText.Faint(true).Render(line))
		} else {
			// Regular line - no styling
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

func helpTextWidth(cmd *cobra.Command) int {
	if cmd == nil {
		return maxHelpTextWidth
	}
	return boundedHelpTextWidth(command.NewStream(cmd.OutOrStdout()).Width())
}

func boundedHelpTextWidth(terminalWidth int) int {
	if terminalWidth <= 0 {
		return maxHelpTextWidth
	}
	return min(terminalWidth, maxHelpTextWidth)
}

// wrapText wraps prose paragraphs at the specified width, respecting word
// boundaries. Blank lines separate paragraphs, and indented lines are left
// untouched.
func wrapText(text string, width int) string {
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	var result []string
	var paragraph []string

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		result = append(result, wrapTextLine(strings.Join(paragraph, " "), width)...)
		paragraph = nil
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flushParagraph()
			result = append(result, "")
			continue
		}

		if isPreformattedLine(line) {
			flushParagraph()
			result = append(result, line)
			continue
		}

		paragraph = append(paragraph, strings.TrimSpace(line))
	}
	flushParagraph()

	return strings.Join(result, "\n")
}

func wrapTextLine(line string, width int) []string {
	if lipgloss.Width(line) <= width {
		return []string{line}
	}

	tokens := wrapTextTokenPattern.FindAllString(line, -1)
	if len(tokens) == 0 {
		return []string{line}
	}

	var result []string
	currentLine := ""
	currentWidth := 0

	for _, token := range tokens {
		tokenIsSpace := strings.TrimSpace(token) == ""
		tokenWidth := lipgloss.Width(token)

		if tokenIsSpace {
			if currentLine == "" {
				continue
			}
			if currentWidth+tokenWidth <= width {
				currentLine += token
				currentWidth += tokenWidth
			}
			continue
		}

		if currentLine != "" && currentWidth+tokenWidth > width {
			result = append(result, strings.TrimRight(currentLine, " \t"))
			currentLine = token
			currentWidth = tokenWidth
		} else {
			currentLine += token
			currentWidth += tokenWidth
		}
	}
	if currentLine != "" {
		result = append(result, strings.TrimRight(currentLine, " \t"))
	}

	return result
}

func isPreformattedLine(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

// trimTrailingPeriod removes a trailing period from text
func trimTrailingPeriod(text string) string {
	return strings.TrimSuffix(text, ".")
}

func hasVisibleGroupCommands(cmd *cobra.Command, groupID string) bool {
	if cmd == nil || groupID == "" {
		return false
	}
	for _, sub := range cmd.Commands() {
		if sub.GroupID == groupID && (sub.IsAvailableCommand() || sub.Name() == "help") {
			return true
		}
	}
	return false
}

// CustomHelpTemplate defines a custom help output format
// Format order:
// 0. Version (dimmed)
// 1. Short description
// 2. USAGE
// 3. SUBCOMMANDS
// 4. FLAGS (local and inherited rendered in one merged section)
// 5. EXAMPLES
// 6. DETAILS (full long description)
var CustomHelpTemplate = `{{cliVersion}}

{{with .Short}}{{.}}

{{end}}{{if or .Runnable .HasSubCommands}}` + style.Title.Render("USAGE") + `
  {{.CommandPath | boldText}}{{getUsageArgs .Use}}{{if .HasAvailableSubCommands}}{{if .Runnable}} [subcommand]{{else}} <subcommand>{{end}}{{end}} [flags]

{{end}}{{if .HasAvailableSubCommands}}` + style.Title.Render("SUBCOMMANDS") + `

{{- range $group := .Groups}}{{if and $group.Title (hasVisibleGroupCommands $ $group.ID)}}
{{$group.Title | groupHeader}}{{range $cmd := $.Commands}}{{if and (eq $cmd.GroupID $group.ID) (or $cmd.IsAvailableCommand (eq $cmd.Name "help"))}}
  {{rpad $cmd.Name 20 | boldText}} {{trimPeriod $cmd.Short}}{{end}}{{end}}
{{end}}{{- end -}}
{{- $hasUngrouped := false -}}
{{- range .Commands -}}{{if and (or .IsAvailableCommand (eq .Name "help")) (not .GroupID)}}{{$hasUngrouped = true}}{{end}}{{end -}}
{{if and $hasUngrouped (eq .CommandPath "render")}}
{{"Additional Commands:" | groupHeader}}{{end}}{{range .Commands}}{{if and (or .IsAvailableCommand (eq .Name "help")) (not .GroupID)}}
  {{rpad .Name 20 | boldText}} {{trimPeriod .Short}}{{end}}{{end}}

{{end}}{{if or .HasAvailableLocalFlags .HasAvailableInheritedFlags}}` + style.Title.Render("FLAGS") + `
{{combinedFlagUsages .LocalFlags .InheritedFlags}}
{{end}}{{if .Example}}` + style.Title.Render("EXAMPLES") + `
{{formatExamples .Example}}

{{end}}{{if .Long}}{{if ne .Long .Short}}` + style.Title.Render("DETAILS") + `
{{wrapText .Long (helpTextWidth .)}}

{{end}}{{end}}{{if .HasAvailableSubCommands}}Use "{{.CommandPath}}{{if .Runnable}} [subcommand]{{else}} <subcommand>{{end}} --help" for more information about a command.
{{end}}`

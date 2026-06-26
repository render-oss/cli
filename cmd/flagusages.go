package cmd

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/render-oss/cli/pkg/style"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	placeholderEnvIDs = "ENV_IDS"

	flagPlaceholderAnnotation = "render.placeholder"
)

// setAnnotationBestEffort applies annotation metadata without failing command initialization.
// Returns true when annotation was applied successfully, false otherwise.
func setAnnotationBestEffort(flags *pflag.FlagSet, flagName, key string, values []string) bool {
	if flags == nil {
		return false
	}
	if flags.Lookup(flagName) == nil {
		return false
	}
	return flags.SetAnnotation(flagName, key, values) == nil
}

// setFlagPlaceholder applies help-output placeholder metadata to a flag.
// Panics when called with an invalid flag name so command wiring mistakes (e.g., wrong flag name) fail loudly in dev and test
func setFlagPlaceholder(flags *pflag.FlagSet, flagName, placeholder string) {
	if flags == nil {
		panic("setFlagPlaceholder called with nil flag set")
	}
	if flags.Lookup(flagName) == nil {
		panic(fmt.Sprintf("setFlagPlaceholder called with unknown flag %q", flagName))
	}
	_ = flags.SetAnnotation(flagName, flagPlaceholderAnnotation, []string{placeholder})
}

// setAllFlagPlaceholders applies help-output placeholder metadata to every local value flag.
// Panics when any visible value flag is missing a placeholder or any placeholder names an invalid flag.
// These panics happen during command construction, so they are intended as a dev and test aid.
func setAllFlagPlaceholders(cmd *cobra.Command, placeholders map[string]string) {
	if cmd == nil {
		panic("setAllFlagPlaceholders called with nil command")
	}
	if len(placeholders) == 0 {
		panic("setAllFlagPlaceholders called with no placeholders")
	}
	for flagName, placeholder := range placeholders {
		setFlagPlaceholder(cmd.Flags(), flagName, placeholder)
	}
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden || isBoolFlag(flag) {
			return
		}
		if _, ok := placeholders[flag.Name]; !ok {
			panic(fmt.Sprintf("setAllFlagPlaceholders missing placeholder for flag %q", flag.Name))
		}
	})
}

func isBoolFlag(flag *pflag.Flag) bool {
	switch flag.Value.Type() {
	case "bool", "boolfunc":
		return true
	default:
		return false
	}
}

func placeholderFromAnnotation(flag *pflag.Flag) (string, bool) {
	if flag == nil || flag.Annotations == nil {
		return "", false
	}
	values, ok := flag.Annotations[flagPlaceholderAnnotation]
	if !ok || len(values) == 0 || values[0] == "" {
		return "", false
	}
	return values[0], true
}

// Intentionally hide only noisy "empty" defaults; keep informative zero values (e.g. 0, 0s).
func isZeroDefaultValue(flag *pflag.Flag) bool {
	switch flag.DefValue {
	case "false", "", "[]":
		return true
	default:
		return false
	}
}

func formatNoOptDefault(flag *pflag.Flag) string {
	if flag.NoOptDefVal == "" {
		return ""
	}

	switch flag.Value.Type() {
	case "string":
		return fmt.Sprintf("[=\"%s\"]", flag.NoOptDefVal)
	case "bool", "boolfunc":
		if flag.NoOptDefVal == "true" {
			return ""
		}
		return fmt.Sprintf("[=%s]", flag.NoOptDefVal)
	case "count":
		if flag.NoOptDefVal == "+1" {
			return ""
		}
		return fmt.Sprintf("[=%s]", flag.NoOptDefVal)
	default:
		return fmt.Sprintf("[=%s]", flag.NoOptDefVal)
	}
}

func formatDefaultAndDeprecation(flag *pflag.Flag) string {
	parts := ""

	if !isZeroDefaultValue(flag) {
		if flag.Value.Type() == "string" {
			parts += fmt.Sprintf(" (default: %q)", flag.DefValue)
		} else {
			parts += fmt.Sprintf(" (default: %s)", flag.DefValue)
		}
	}

	if flag.Deprecated != "" {
		parts += fmt.Sprintf(" (DEPRECATED: %s)", flag.Deprecated)
	}

	return parts
}

// getDescriptiveTypeName returns a more descriptive type name for specific flags
func getDescriptiveTypeName(flag *pflag.Flag, varname string) string {
	if customName, ok := placeholderFromAnnotation(flag); ok {
		return customName
	}

	// Default to the original varname
	return varname
}

// CombinedFlagUsages formats both local and inherited flags with consistent padding
func CombinedFlagUsages(localFlags, inheritedFlags *pflag.FlagSet) string {
	type flagInfo struct {
		name                 string
		varname              string
		usage                string
		suffix               string
		defaultAndDeprecated string
	}

	var allFlags []flagInfo
	maxFlagLen := 0

	// Collect all flags (local first, then inherited)
	collectFlags := func(flags *pflag.FlagSet) {
		flags.VisitAll(func(flag *pflag.Flag) {
			if flag.Hidden {
				return
			}

			name := ""
			if flag.Shorthand != "" && flag.ShorthandDeprecated == "" {
				name = fmt.Sprintf("  -%s, --%s", flag.Shorthand, flag.Name)
			} else {
				name = fmt.Sprintf("      --%s", flag.Name)
			}

			varname, usage := pflag.UnquoteUsage(flag)
			if flag.Name == "help" {
				usage = "Show help for this command"
			}

			// Get descriptive type name
			if varname != "" {
				varname = getDescriptiveTypeName(flag, varname)
			}

			suffix := formatNoOptDefault(flag)

			// Calculate max display width including type placeholder
			fullLength := lipgloss.Width(name)
			if varname != "" {
				fullLength += lipgloss.Width(fmt.Sprintf(" <%s>", varname))
			}
			fullLength += lipgloss.Width(suffix)

			if fullLength > maxFlagLen {
				maxFlagLen = fullLength
			}

			allFlags = append(allFlags, flagInfo{
				name:                 name,
				varname:              varname,
				usage:                usage,
				suffix:               suffix,
				defaultAndDeprecated: formatDefaultAndDeprecation(flag),
			})
		})
	}

	if localFlags != nil {
		collectFlags(localFlags)
	}
	if inheritedFlags != nil {
		collectFlags(inheritedFlags)
	}

	// Format all flags with consistent padding
	buf := new(bytes.Buffer)
	const padding = 3

	for _, flag := range allFlags {
		// Parse and format flag name with selective bolding
		// Don't bold the comma and space between short and long flags
		flagName := flag.name
		if strings.Contains(flagName, ", --") {
			// Split on the comma to bold separately
			parts := strings.SplitN(flagName, ", ", 2)
			buf.WriteString(style.Bold(parts[0]))
			buf.WriteString(", ")
			buf.WriteString(style.Bold(parts[1]))
		} else {
			// No comma, just bold the whole thing
			buf.WriteString(style.Bold(flagName))
		}

		// Add type placeholder if exists
		if flag.varname != "" {
			typeStr := fmt.Sprintf(" <%s>", flag.varname)
			buf.WriteString(typeStr)
		}
		if flag.suffix != "" {
			buf.WriteString(flag.suffix)
		}

		// Calculate current display width
		currentLen := lipgloss.Width(flag.name)
		if flag.varname != "" {
			currentLen += lipgloss.Width(fmt.Sprintf(" <%s>", flag.varname))
		}
		currentLen += lipgloss.Width(flag.suffix)

		// Add padding to align descriptions
		spacesToAdd := maxFlagLen - currentLen + padding
		buf.WriteString(strings.Repeat(" ", spacesToAdd))

		// Remove trailing period from usage description
		usage := strings.TrimSuffix(flag.usage, ".")
		buf.WriteString(usage)
		buf.WriteString(flag.defaultAndDeprecated)
		buf.WriteString("\n")
	}

	return buf.String()
}

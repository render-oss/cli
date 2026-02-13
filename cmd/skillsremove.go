package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/skills"
	renderstyle "github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
)

var skillsRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove installed Render skills from AI coding tools",
	Long: `Remove previously installed Render skills from detected AI coding tools.

By default an interactive prompt lets you pick which skills to remove.
Use --skill and --all flags to skip the prompts.

Use --scope to remove from a specific scope (user or project).`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		skillFilter, _ := cmd.Flags().GetStringSlice("skill")
		toolFilter, _ := cmd.Flags().GetString("tool")
		removeAll, _ := cmd.Flags().GetBool("all")
		scopeFilter, _ := cmd.Flags().GetString("scope")

		// Parse scope if provided
		var scope skills.Scope
		if scopeFilter != "" {
			var err error
			scope, err = skills.ParseScope(scopeFilter)
			if err != nil {
				return err
			}
		}

		// Non-interactive path: flags provided.
		if removeAll || len(skillFilter) > 0 {
			return nonInteractiveSkillsRemove(cmd, skillFilter, toolFilter, removeAll, scope)
		}

		// Interactive path: push TUI view onto the stack.
		// We push directly (not via AddToStackFunc) because skills are
		// purely local — there's no CLI command string to copy.
		ctx := cmd.Context()
		stack := tui.GetStackFromContext(ctx)
		stack.Push(tui.ModelWithCmd{
			Model:      views.NewSkillsRemoveView(scope),
			Breadcrumb: "Remove Skills",
		})
		return nil
	},
}

func init() {
	skillsCmd.AddCommand(skillsRemoveCmd)
	skillsRemoveCmd.Flags().StringSlice("skill", nil, "remove specific skills (e.g. --skill render-deploy --skill render-debug)")
	skillsRemoveCmd.Flags().String("tool", "", "remove from a specific tool only (claude, codex, opencode, cursor)")
	skillsRemoveCmd.Flags().Bool("all", false, "remove all installed Render skills")
	skillsRemoveCmd.Flags().String("scope", "", "remove from specific scope: user or project")
}

// nonInteractiveSkillsRemove runs the remove flow without prompts.
func nonInteractiveSkillsRemove(cmd *cobra.Command, skillFilter []string, toolFilter string, removeAll bool, scope skills.Scope) error {
	// Default to user scope when --scope is not specified, matching the
	// install path. Without this, GetScopedSkillsDir would treat the empty
	// string as project scope with an empty repoRoot, producing a relative
	// path like ".claude/skills" instead of the user's home directory.
	if scope == "" {
		scope = skills.ScopeUser
	}

	// Get repo root for project scope
	var repoRoot string
	if scope == skills.ScopeProject {
		var err error
		repoRoot, err = skills.GetRepoRoot()
		if err != nil {
			return fmt.Errorf("project scope requires a git repository: %w", err)
		}
	}
	state, err := skills.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load skills state: %w", err)
	}

	detectedTools, err := skills.DetectTools()
	if err != nil {
		return fmt.Errorf("failed to detect tools: %w", err)
	}

	if len(detectedTools) == 0 {
		return fmt.Errorf("no supported AI coding tools detected")
	}

	if !state.HasSelections() {
		allInstalled, toolNames, _ := skills.ScanInstalledState(detectedTools)
		if len(allInstalled) == 0 {
			command.Println(cmd, "No installed skills found.")
			return nil
		}
		state.Tools = toolNames
		state.Skills = allInstalled
	}

	// Filter tools.
	detectedMap := make(map[string]skills.Tool, len(detectedTools))
	for _, t := range detectedTools {
		detectedMap[t.Name] = t
	}
	var selectedTools []skills.Tool
	if toolFilter != "" {
		selectedTools = skills.FilterTools(detectedTools, toolFilter)
		if len(selectedTools) == 0 {
			return fmt.Errorf("no installed tool matching %q found", toolFilter)
		}
	} else {
		for _, name := range state.Tools {
			if t, ok := detectedMap[name]; ok {
				selectedTools = append(selectedTools, t)
			}
		}
	}
	if len(selectedTools) == 0 {
		return fmt.Errorf("no matching tools found")
	}

	// Determine which skills to remove. Use directory names because
	// RemoveSkills matches against filesystem directory names.
	// Only consider skills matching the current scope.
	var toRemove []string
	if removeAll {
		for _, sk := range state.Skills {
			if sk.EffectiveScope() == scope {
				toRemove = append(toRemove, sk.EffectiveDirName())
			}
		}
	} else {
		// Build lookup maps for both Name and DirName so --skill flags
		// work whether the user supplies a frontmatter name or a directory name.
		// Only include skills at the selected scope.
		nameToDir := make(map[string]string, len(state.Skills))
		for _, sk := range state.Skills {
			if sk.EffectiveScope() == scope {
				nameToDir[sk.Name] = sk.EffectiveDirName()
				nameToDir[sk.EffectiveDirName()] = sk.EffectiveDirName()
			}
		}
		for _, name := range skillFilter {
			if dirName, ok := nameToDir[name]; ok {
				toRemove = append(toRemove, dirName)
			} else {
				command.Println(cmd, "Skill %s is not installed at %s scope, skipping", renderstyle.Bold(name), scope)
			}
		}
	}

	if len(toRemove) == 0 {
		command.Println(cmd, "No skills to remove.")
		return nil
	}

	// Remove.
	successCount := 0
	for _, t := range selectedTools {
		// Get the appropriate skills directory based on scope
		skillsDir := skills.GetScopedSkillsDir(t, scope, repoRoot)

		if err := skills.RemoveSkills(skillsDir, toRemove); err != nil {
			command.Println(cmd, "Error: %s: %s", t.Name, err)
			continue
		}
		command.Println(cmd, "Removed %d skill(s) from %s", len(toRemove), skills.ShortenPath(skillsDir))
		successCount++
	}

	if successCount == 0 {
		return fmt.Errorf("failed to remove skills from any tool")
	}

	// Update state — only remove skills matching the current scope.
	skills.UpdateStateAfterRemoval(state, toRemove, scope)

	return nil
}

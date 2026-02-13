package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/skills"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
)

// SkillsInstallInput holds the CLI input for skills install.
type SkillsInstallInput struct {
	Tool   string   `cli:"tool"`
	Skills []string `cli:"skill"`
	DryRun bool     `cli:"dry-run"`
	Scope  string   `cli:"scope"`
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Render skills to AI coding tools",
	Long: `Install Render agent skills from https://github.com/render-oss/skills to
detected AI coding tools.

Supported tools: Claude Code, Codex, OpenCode, Cursor.

Skills can be installed at two scopes:
  - user:    Install to ~/.{tool}/skills/ (default, current user only)
  - project: Install to ./.{tool}/skills/ (committed to git, all collaborators)

By default an interactive prompt lets you pick scope, tools, and skills.
Use --scope, --tool, and --skill flags to skip the prompts (useful for CI).`,
	SilenceUsage: true,
}

func init() {
	skillsCmd.AddCommand(skillsInstallCmd)
	skillsInstallCmd.Flags().String("tool", "", "install to a specific tool only (claude, codex, opencode, cursor)")
	skillsInstallCmd.Flags().StringSlice("skill", nil, "install specific skills only (e.g. --skill render-deploy --skill render-debug)")
	skillsInstallCmd.Flags().Bool("dry-run", false, "show what would be installed without making changes")
	skillsInstallCmd.Flags().String("scope", "", "installation scope: user (default) or project")

	skillsInstallCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input SkillsInstallInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		// Parse scope if provided
		var scope skills.Scope
		if input.Scope != "" {
			var err error
			scope, err = skills.ParseScope(input.Scope)
			if err != nil {
				return err
			}
		}

		// Non-interactive path: use command.NonInteractive
		if nonInteractive, err := command.NonInteractive(cmd, func() (*skills.InstallResult, error) {
			return skills.Install(skills.InstallInput{
				ToolFilter:  input.Tool,
				SkillFilter: input.Skills,
				DryRun:      input.DryRun,
				Scope:       scope,
			})
		}, func(r *skills.InstallResult) string {
			action := "Installed"
			if r.DryRun {
				action = "Would install"
			}
			return text.FormatStringF("%s %d skill(s) to %d tool(s)", action, len(r.Skills), len(r.Tools))
		}); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		// Interactive path: launch TUI with pre-populated input
		interactiveSkillsInstall(cmd, input, scope)
		return nil
	}
}

// interactiveSkillsInstall launches the TUI, skipping steps based on provided flags.
func interactiveSkillsInstall(cmd *cobra.Command, input SkillsInstallInput, scope skills.Scope) {
	ctx := cmd.Context()
	stack := tui.GetStackFromContext(ctx)

	// Pass input to the view so it can skip steps if flags are provided
	stack.Push(tui.ModelWithCmd{
		Model: views.NewSkillsInstallView(views.SkillsInstallViewInput{
			ToolFilter:  input.Tool,
			SkillFilter: input.Skills,
			Scope:       scope,
		}),
		Breadcrumb: "Install Skills",
	})
}

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/skills"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
)

var skillsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update previously installed Render skills",
	Long: `Re-install Render skills using the tool and skill selections saved by
a previous "render skills install" run.

This fetches the latest version of each selected skill from the skills
repository, compares with installed versions, and updates any that have changed.

Use --scope to update skills at a specific scope (user or project).`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		scopeFilter, _ := cmd.Flags().GetString("scope")

		var scope skills.Scope
		if scopeFilter != "" {
			var err error
			scope, err = skills.ParseScope(scopeFilter)
			if err != nil {
				return err
			}
		}

		// Push TUI view onto the stack.
		// We push directly (not via AddToStackFunc) because skills are
		// purely local — there's no CLI command string to copy.
		ctx := cmd.Context()
		stack := tui.GetStackFromContext(ctx)
		stack.Push(tui.ModelWithCmd{
			Model:      views.NewSkillsUpdateView(force, scope),
			Breadcrumb: "Update Skills",
		})
		return nil
	},
}

func init() {
	skillsCmd.AddCommand(skillsUpdateCmd)
	skillsUpdateCmd.Flags().Bool("force", false, "reinstall all skills even if already up to date")
	skillsUpdateCmd.Flags().String("scope", "", "update skills at specific scope: user or project")
}

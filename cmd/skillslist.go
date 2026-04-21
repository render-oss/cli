package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/skills"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
)

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed Render skills and detected tools",
	Long: `List installed Render skills and the AI tools they've been installed in. This reads from local state only, so the command doesn't require network access.

Use --scope to filter by installation scope (user or project).`,
	Example: `  # List all installed skills
  render skills list

  # List project-scoped skills only
  render skills list --scope project`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
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
			Model:      views.NewSkillsListView(scope),
			Breadcrumb: "List Skills",
		})
		return nil
	},
}

func init() {
	skillsCmd.AddCommand(skillsListCmd)
	skillsListCmd.Flags().String("scope", "", "Filter skills by installation scope (user or project)")
	setAnnotationBestEffort(skillsListCmd.Flags(), "scope", command.FlagPlaceholderAnnotation, []string{"SCOPE"})
}

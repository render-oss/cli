package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/skills"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
)

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed Render skills and detected tools",
	Long: `Show which Render skills are currently installed and which AI coding tools
they are installed to. This reads from local state only — no network
access is required.

Use --scope to filter by installation scope (user or project).`,
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
	skillsListCmd.Flags().String("scope", "", "filter by scope: user or project")
}

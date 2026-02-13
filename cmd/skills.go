package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage Render agent skills for AI coding tools",
	Long: `Install and manage Render agent skills for AI coding tools such as
Claude Code, Codex, OpenCode, and Cursor.

Skills add deployment, debugging, and monitoring capabilities to your
AI coding assistant.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		stack := tui.GetStackFromContext(ctx)

		commands := []views.PaletteCommand{
			{
				Name:        "list",
				Description: "List installed skills and detected tools",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return pushSkillsView(ctx, views.NewSkillsListView(""), "List Skills")
				},
			},
			{
				Name:        "install",
				Description: "Install skills to AI coding tools",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return pushSkillsView(ctx, views.NewSkillsInstallView(views.SkillsInstallViewInput{}), "Install Skills")
				},
			},
			{
				Name:        "update",
				Description: "Update previously installed skills",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return pushSkillsView(ctx, views.NewSkillsUpdateView(false, ""), "Update Skills")
				},
			},
			{
				Name:        "remove",
				Description: "Remove installed skills from tools",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return pushSkillsView(ctx, views.NewSkillsRemoveView(""), "Remove Skills")
				},
			},
		}

		palette := views.NewPaletteView(ctx, commands)
		stack.Push(tui.ModelWithCmd{
			Model:      palette,
			Breadcrumb: "Skills",
		})
		return nil
	},
}

func init() {
	rootCmd.AddCommand(skillsCmd)
}

// pushSkillsView pushes a skills view onto the TUI stack.
// Skills views are pushed directly (not via AddToStackFunc) because
// they are purely local — there's no CLI command string to copy.
func pushSkillsView(ctx context.Context, model tea.Model, breadcrumb string) tea.Cmd {
	stack := tui.GetStackFromContext(ctx)
	return stack.Push(tui.ModelWithCmd{
		Model:      model,
		Breadcrumb: breadcrumb,
	})
}

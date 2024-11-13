package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/tui"
	"github.com/renderinc/cli/pkg/tui/views"
)

var projectCmd = &cobra.Command{
	Use:   "projects",
	Short: "List projects",
	Long: `List projects for the active workspace.
In interactive mode you can view the environments for a project.`,
	GroupID: GroupManagement.ID,
}

var InteractiveProjectList = func(ctx context.Context) {
	command.AddToStackFunc(
		ctx,
		projectCmd,
		"Projects",
		&views.ProjectInput{},
		views.NewProjectList(ctx,
			func(ctx context.Context, p *client.Project) tea.Cmd {
				return InteractiveEnvironment(ctx, views.EnvironmentInput{
					ProjectID: p.Id,
				}, p.Name)
			},
			tui.WithCustomOptions[*client.Project]([]tui.CustomOption{
				WithWorkspaceSelection(ctx),
			}),
		))
}

func init() {
	rootCmd.AddCommand(projectCmd)

	projectCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if nonInteractive, err := command.NonInteractive(
			cmd,
			func() (any, error) {
				return views.LoadProjects(cmd.Context(), views.ProjectInput{})
			},
			nil,
		); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		InteractiveProjectList(cmd.Context())
		return nil
	}
}

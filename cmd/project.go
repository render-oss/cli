package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/renderinc/render-cli/pkg/tui/views"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "projects",
	Short: "List projects",
	Long: `List projects for the currently set workspace.
In interactive mode you can view the environments for a project.`,
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
			{
				Key:   "w",
				Title: "Change Workspace",
				Function: func(row btable.Row) tea.Cmd {
					return InteractiveWorkspaceSet(ctx, views.ListWorkspaceInput{})
				},
			},
		}),
	))
}

func init() {
	rootCmd.AddCommand(projectCmd)

	projectCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if nonInteractive, err := command.NonInteractive(cmd.Context(), cmd, func() (any, error) {
			return views.LoadProjects(cmd.Context(), views.ProjectInput{})
		}, nil); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		InteractiveProjectList(cmd.Context())
		return nil
	}
}

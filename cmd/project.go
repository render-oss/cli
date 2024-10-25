package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/project"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "projects",
	Short: "List projects",
	Long: `List projects for the currently set workspace.
In interactive mode you can view the environments for a project.`,
}

var InteractiveProject = command.Wrap(projectCmd, loadProjects, renderProjects, nil)

type ProjectInput struct{}

func loadProjects(ctx context.Context, _ ProjectInput) ([]*client.Project, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	projectRepo := project.NewRepo(c)
	return projectRepo.ListProjects(ctx)
}

func renderProjects(ctx context.Context, loadData func(ProjectInput) tui.TypedCmd[[]*client.Project], in ProjectInput) (tea.Model, error) {
	columns := []btable.Column{
		btable.NewColumn("ID", "ID", 25).WithFiltered(true),
		btable.NewFlexColumn("Name", "Name", 40).WithFiltered(true),
	}

	createRowFunc := func(p *client.Project) btable.Row {
		return btable.NewRow(btable.RowData{
			"ID":      p.Id,
			"Name":    p.Name,
			"project": p, // this will be hidden in the UI, but will be used to get the project when selected
		})
	}

	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		p, ok := rows[0].Data["project"].(*client.Project)
		if !ok {
			return nil
		}

		return InteractiveEnvironment(ctx, EnvironmentInput{
			ProjectID: p.Id,
		})
	}

	customOptions := []tui.CustomOption{
		{
			Key:   "w",
			Title: "Change Workspace",
			Function: func(row btable.Row) tea.Cmd {
				return InteractiveWorkspaceSet(ctx, ListWorkspaceInput{})
			},
		},
	}

	t := tui.NewTable(
		columns,
		loadData(in),
		createRowFunc,
		onSelect,
		tui.WithCustomOptions[*client.Project](customOptions),
	)

	return t, nil
}

func init() {
	rootCmd.AddCommand(projectCmd)

	projectCmd.RunE = func(cmd *cobra.Command, args []string) error {
		InteractiveProject(cmd.Context(), ProjectInput{})
		return nil
	}
}

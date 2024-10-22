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

var InteractiveProject = command.Wrap(projectCmd, loadProjects, renderProjects)

type ProjectInput struct{}

func loadProjects(ctx context.Context, _ ProjectInput) ([]*client.Project, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	projectRepo := project.NewRepo(c)
	return projectRepo.ListProjects(ctx)
}

func renderProjects(ctx context.Context, loadData func(ProjectInput) ([]*client.Project, error), in ProjectInput) (tea.Model, error) {
	columns := []btable.Column{
		btable.NewColumn("ID", "ID", 25).WithFiltered(true),
		btable.NewColumn("Name", "Name", 40).WithFiltered(true),
	}

	loadDataFunc := func() ([]*client.Project, error) {
		return loadData(in)
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

		return selectProject(ctx)(p)
	}

	customOptions := []tui.CustomOption{
		{
			Key:   "w",
			Title: "Change Workspace",
			Function: func(row btable.Row) tea.Cmd {
				return InteractiveWorkspace(ctx, ListWorkspaceInput{})
			},
		},
	}

	t := tui.NewTable(
		columns,
		loadDataFunc,
		createRowFunc,
		onSelect,
		tui.WithCustomOptions[*client.Project](customOptions),
	)

	return t, nil
}

func selectProject(ctx context.Context) func(*client.Project) tea.Cmd {
	return func(p *client.Project) tea.Cmd {
		commands := []PaletteCommand{
			{
				Name:        "environments",
				Description: "View environments in project",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return InteractiveEnvironment(ctx, EnvironmentInput{
						ProjectID: p.Id,
					})
				},
			},
		}

		return InteractiveCommandPalette(ctx, PaletteCommandInput{
			Commands: commands,
		})
	}
}

func init() {
	rootCmd.AddCommand(projectCmd)

	projectCmd.RunE = func(cmd *cobra.Command, args []string) error {
		InteractiveProject(cmd.Context(), ProjectInput{})
		return nil
	}
}

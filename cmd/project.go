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
	Use:   "project",
	Short: "List projects",
	Long: `List projects for the currently set workspace.
In interactive mode you can view the environments for a project.`,
}

var InteractiveProject = command.Wrap(projectCmd, loadProjects, renderProjects)

type ProjectInput struct{}

func (p ProjectInput) String() []string {
	return []string{}
}

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

	rows, err := loadProjectRows(loadData, in)
	if err != nil {
		return nil, err
	}

	onSelect := func(data []btable.Row) tea.Cmd {
		if len(data) == 0 || len(data) > 1 {
			return nil
		}

		p, ok := data[0].Data["project"].(client.Project)
		if !ok {
			return nil
		}

		return selectProject(ctx)(&p)
	}

	reInitFunc := func(tableModel *tui.NewTable) tea.Cmd {
		return func() tea.Msg {
			rows, err := loadProjectRows(loadData, in)
			if err != nil {
				return tui.ErrorMsg{Err: err}
			}
			tableModel.UpdateRows(rows)
			return nil
		}
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

	t := tui.NewNewTable(
		columns,
		rows,
		onSelect,
		tui.WithCustomOptions(customOptions),
		tui.WithOnReInit(reInitFunc),
	)

	return t, nil
}

func loadProjectRows(loadData func(input ProjectInput) ([]*client.Project, error), in ProjectInput) ([]btable.Row, error) {
	projects, err := loadData(in)
	if err != nil {
		return nil, err
	}

	var rows []btable.Row
	for _, p := range projects {
		rows = append(rows, btable.NewRow(btable.RowData{
			"ID":      p.Id,
			"Name":    p.Name,
			"project": p, // this will be hidden in the UI, but will be used to get the project when selected
		}))
	}
	return rows, nil
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

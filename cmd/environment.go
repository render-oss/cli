package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

var environmentCmd = &cobra.Command{
	Use:   "environment [projectID]",
	Short: "List environments",
	Long: `List environments for the currently set workspace and the specified project.
In interactive mode you can view the services for an environment.`,
}

var InteractiveEnvironment = command.Wrap(environmentCmd, loadEnvironments, renderEnvironments)

type EnvironmentInput struct {
	ProjectID string
}

func (e EnvironmentInput) String() []string {
	return []string{}
}

func (e EnvironmentInput) ToParams() *client.ListEnvironmentsParams {
	return &client.ListEnvironmentsParams{
		ProjectId: []string{e.ProjectID},
	}
}

func loadEnvironments(ctx context.Context, in EnvironmentInput) ([]*client.Environment, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	environmentRepo := environment.NewRepo(c)

	return environmentRepo.ListEnvironments(ctx, in.ToParams())
}

func selectEnvironment(ctx context.Context) func(*client.Environment) tea.Cmd {
	return func(r *client.Environment) tea.Cmd {
		commands := []PaletteCommand{
			{
				Name:        "services",
				Description: "View services in environment",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return InteractiveServices(ctx, ListResourceInput{
						EnvironmentID: r.Id,
					})
				},
			},
		}

		return InteractiveCommandPalette(ctx, PaletteCommandInput{
			Commands: commands,
		})
	}
}

func renderEnvironments(ctx context.Context, loadData func(EnvironmentInput) ([]*client.Environment, error), input EnvironmentInput) (tea.Model, error) {
	columns := []btable.Column{
		btable.NewColumn("ID", "ID", 25).WithFiltered(true),
		btable.NewFlexColumn("Name", "Name", 3).WithFiltered(true),
		btable.NewFlexColumn("Project", "Project", 3).WithFiltered(true),
		btable.NewFlexColumn("Protected", "Protected", 2).WithFiltered(true),
	}

	rows, err := loadEnvironmentRows(loadData, input)
	if err != nil {
		return nil, err
	}

	onSelect := func(data []btable.Row) tea.Cmd {
		if len(data) == 0 || len(data) > 1 {
			return nil
		}

		env, ok := data[0].Data["environment"].(*client.Environment)
		if !ok {
			return nil
		}

		return selectEnvironment(ctx)(env)
	}

	reInitFunc := func(tableModel *tui.Table) tea.Cmd {
		return func() tea.Msg {
			rows, err := loadEnvironmentRows(loadData, input)
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

	t := tui.NewTable(
		columns,
		rows,
		onSelect,
		tui.WithCustomOptions(customOptions),
		tui.WithOnReInit(reInitFunc),
	)

	return t, nil
}

func loadEnvironmentRows(loadData func(input EnvironmentInput) ([]*client.Environment, error), in EnvironmentInput) ([]btable.Row, error) {
	environments, err := loadData(in)
	if err != nil {
		return nil, err
	}

	var rows []btable.Row
	for _, env := range environments {
		rows = append(rows, btable.NewRow(btable.RowData{
			"ID":          env.Id,
			"Name":        env.Name,
			"Project":     env.ProjectId,
			"Protected":   string(env.ProtectedStatus),
			"environment": env, // this will be hidden in the UI, but will be used to get the environment when selected
		}))
	}
	return rows, nil
}

func init() {
	rootCmd.AddCommand(environmentCmd)

	environmentCmd.RunE = func(cmd *cobra.Command, args []string) error {
		projectID := args[0]

		InteractiveEnvironment(cmd.Context(), EnvironmentInput{
			ProjectID: projectID,
		})
		return nil
	}
}
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
	Use:   "environments [projectID]",
	Args:  cobra.ExactArgs(1),
	Short: "List environments",
	Long: `List environments for the currently set workspace and the specified project.
In interactive mode you can view the services for an environment.`,
}

var InteractiveEnvironment = command.Wrap(environmentCmd, loadEnvironments, renderEnvironments)

type EnvironmentInput struct {
	ProjectID string `cli:"arg:0"`
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

func renderEnvironments(ctx context.Context, loadData func(EnvironmentInput) ([]*client.Environment, error), input EnvironmentInput) (tea.Model, error) {
	columns := []btable.Column{
		btable.NewColumn("ID", "ID", 25).WithFiltered(true),
		btable.NewFlexColumn("Name", "Name", 3).WithFiltered(true),
		btable.NewFlexColumn("Project", "Project", 3).WithFiltered(true),
		btable.NewFlexColumn("Protected", "Protected", 2).WithFiltered(true),
	}

	loadDataFunc := func() ([]*client.Environment, error) {
		return loadData(input)
	}

	createRowFunc := func(env *client.Environment) btable.Row {
		return btable.NewRow(btable.RowData{
			"ID":          env.Id,
			"Name":        env.Name,
			"Project":     env.ProjectId,
			"Protected":   string(env.ProtectedStatus),
			"environment": env, // this will be hidden in the UI, but will be used to get the environment when selected
		})
	}

	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		env, ok := rows[0].Data["environment"].(*client.Environment)
		if !ok {
			return nil
		}

		return InteractiveServices(ctx, ListResourceInput{
			EnvironmentID: env.Id,
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
		loadDataFunc,
		createRowFunc,
		onSelect,
		tui.WithCustomOptions[*client.Environment](customOptions),
	)

	return t, nil
}

func init() {
	rootCmd.AddCommand(environmentCmd)

	environmentCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input EnvironmentInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		InteractiveEnvironment(cmd.Context(), input)
		return nil
	}
}

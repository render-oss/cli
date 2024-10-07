/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/project"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

// projectCmd represents the project command
var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "List projects",
	Long:  `List projects.`,
}

var InteractiveProject = command.Wrap(projectCmd, loadProjects, renderProjects)

type ProjectInput struct {
}

func (p ProjectInput) String() []string {
	return []string{}
}

func loadProjects(ctx context.Context, _ ProjectInput) ([]*client.Project, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	projectRepo := project.NewRepo(c)

	return projectRepo.ListProjects(ctx)
}

func selectProject(ctx context.Context) func(*client.Project) tea.Cmd {
	return func(r *client.Project) tea.Cmd {
		commands := []PaletteCommand{
			{
				Name:        "environments",
				Description: "View environments in project",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					// TODO: Implement once environments are added
					return func() tea.Msg {
						return nil
					}
				},
			},
		}

		return InteractiveCommandPalette(ctx, PaletteCommandInput{
			Commands: commands,
		})
	}
}

func formatProjectRow(p *client.Project) table.Row {
	// r.ID() must be first because it's used when selecting a row in selectCurrentRow()
	// TODO: make this less brittle
	return []string{p.Id, p.Name}
}

func filterProject(p *client.Project, filter string) bool {
	searchFields := []string{p.Id, p.Name}
	for _, field := range searchFields {
		if strings.Contains(strings.ToLower(field), filter) {
			return true
		}
	}
	return false
}

func renderProjects(ctx context.Context, loadData func(ProjectInput) ([]*client.Project, error), input ProjectInput) (tea.Model, error) {
	columns := []table.Column{
		{Title: "ID", Width: 25},
		{Title: "Name", Width: 40},
	}

	return tui.NewTableModel(
		"resources",
		func() ([]*client.Project, error) {
			return loadData(input)
		},
		formatProjectRow,
		selectProject(ctx),
		columns,
		filterProject,
	), nil
}

func projectOptionSelectWorkspace(ctx context.Context) func(*client.Project) tea.Cmd {
	return func(r *client.Project) tea.Cmd {
		return InteractiveWorkspace(ctx, ListWorkspaceInput{})
	}
}

func init() {
	rootCmd.AddCommand(projectCmd)

	projectCmd.RunE = func(cmd *cobra.Command, args []string) error {
		InteractiveProject(cmd.Context(), ProjectInput{})
		return nil
	}
}

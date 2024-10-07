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
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

// environmentCmd represents the environment command
var environmentCmd = &cobra.Command{
	Use:   "environment",
	Short: "List environments",
	Long:  `List environments.`,
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

func formatEnvironmentRow(p *client.Environment) table.Row {
	// r.ID() must be first because it's used when selecting a row in selectCurrentRow()
	// TODO: make this less brittle
	return []string{p.Id, p.Name, p.ProjectId, string(p.ProtectedStatus)}
}

func filterEnvironment(p *client.Environment, filter string) bool {
	searchFields := []string{p.Id, p.Name}
	for _, field := range searchFields {
		if strings.Contains(strings.ToLower(field), filter) {
			return true
		}
	}
	return false
}

func renderEnvironments(ctx context.Context, loadData func(EnvironmentInput) ([]*client.Environment, error), input EnvironmentInput) (tea.Model, error) {
	columns := []table.Column{
		{Title: "ID", Width: 25},
		{Title: "Name", Width: 40},
		{Title: "Project", Width: 15},
		{Title: "Protected", Width: 10},
	}

	return tui.NewTableModel[*client.Environment](
		"environments",
		func() ([]*client.Environment, error) {
			return loadData(input)
		},
		formatEnvironmentRow,
		selectEnvironment(ctx),
		columns,
		filterEnvironment,
		[]tui.CustomOption[*client.Environment]{
			{
				Key:      "w",
				Title:    "Change Workspace",
				Function: environmentOptionSelectWorkspace(ctx),
			},
		},
	), nil
}

func environmentOptionSelectWorkspace(ctx context.Context) func(*client.Environment) tea.Cmd {
	return func(r *client.Environment) tea.Cmd {
		return InteractiveWorkspace(ctx, ListWorkspaceInput{})
	}
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

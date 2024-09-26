package cmd

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/project"
	"github.com/renderinc/render-cli/pkg/service"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "List and manage services",
	RunE: func(cmd *cobra.Command, args []string) error {
		command.Wrap(cmd, loadServiceData, renderServices)(cmd.Context(), ListServiceInput{})
		return nil
	},
}

func loadServiceData(ctx context.Context, _ ListServiceInput) ([]*service.Model, error) {
	_, serviceService, err := newRepositories()
	if err != nil {
		return nil, err
	}
	return serviceService.ListServices(ctx)
}

type ListServiceInput struct{}

func (l ListServiceInput) String() []string {
	return []string{}
}

func renderServices(ctx context.Context, loadData func(input ListServiceInput) ([]*service.Model, error), in ListServiceInput) (tea.Model, error) {
	columns := []table.Column{
		{Title: "Project", Width: 25},
		{Title: "Environment", Width: 25},
		{Title: "ID", Width: 25},
		{Title: "Name", Width: 40},
	}

	return tui.NewTableModel[*service.Model](
		"services",
		func() ([]*service.Model, error) {
			return loadData(in)
		},
		formatServiceRow,
		selectService(ctx),
		columns,
		filterService,
		[]tui.CustomOption[*service.Model]{
			{
				Key:      "w",
				Title:    "Change Workspace",
				Function: serviceOptionSelectWorkspace(ctx),
			},
		},
	), nil
}

func formatServiceRow(s *service.Model) table.Row {
	projectName := ""
	if s.Project != nil {
		projectName = s.Project.Name
	}

	environmentName := ""
	if s.Environment != nil {
		environmentName = s.Environment.Name
	}

	return []string{projectName, environmentName, s.Service.Id, s.Service.Name}
}

func selectService(ctx context.Context) func(*service.Model) tea.Cmd {
	return func(s *service.Model) tea.Cmd {
		return InteractiveCommandPalette(ctx, PaletteCommandInput{
			Commands: []PaletteCommand{
				{
					Name:        "logs",
					Description: "View service logs",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveLogs(ctx, LogInput{ResourceIDs: []string{s.Service.Id}})
					},
				},
			},
		})
	}
}

func filterService(s *service.Model, filter string) bool {
	projectName := ""
	if s.Project != nil {
		projectName = s.Project.Name
	}
	envName := ""
	if s.Environment != nil {
		envName = s.Environment.Name
	}

	searchFields := []string{s.Service.Id, s.Service.Name, projectName, envName}
	for _, field := range searchFields {
		if strings.Contains(strings.ToLower(field), filter) {
			return true
		}
	}
	return false
}

func newRepositories() (*service.Repo, *service.Service, error) {
	httpClient := http.DefaultClient
	host := os.Getenv("RENDER_HOST")
	apiKey := os.Getenv("RENDER_API_KEY")

	c, err := client.ClientWithAuth(httpClient, host, apiKey)
	if err != nil {
		return nil, nil, err
	}

	serviceRepo, err := service.NewRepo(c), nil
	if err != nil {
		return nil, nil, err
	}

	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)
	serviceService := service.NewService(serviceRepo, environmentRepo, projectRepo)

	return serviceRepo, serviceService, nil
}

func serviceOptionSelectWorkspace(ctx context.Context) func(*service.Model) tea.Cmd {
	return func(s *service.Model) tea.Cmd {
		return InteractiveWorkspace(ctx, ListWorkspaceInput{})
	}
}

func init() {
	rootCmd.AddCommand(servicesCmd)
}

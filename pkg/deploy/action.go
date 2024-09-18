package deploy

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/services"
)

type Action struct {
	Service *client.Service
	Repo    *services.ServiceRepo
}

func (da *Action) Execute() (tea.Model, tea.Cmd) {
	return NewModel(da.Service, da.Repo), func() tea.Msg {
		tea.Printf("Deploying service %s...\n", da.Service.Name)
		deploy, err := da.Repo.DeployService(context.Background(), da.Service)
		if err != nil {
			return errMsg(fmt.Errorf("error deploying service: %v", err))
		}
		return deployedMsg(deploy)
	}
}

type Model struct {
	service *client.Service
	repo    *services.ServiceRepo
	deploy  *client.Deploy
	err     error
}

func NewModel(service *client.Service, repo *services.ServiceRepo) *Model {
	return &Model{
		service: service,
		repo:    repo,
	}
}

type deployedMsg *client.Deploy
type errMsg error

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case deployedMsg:
		m.deploy = msg
	case errMsg:
		m.err = msg
	}
	return m, nil
}

func (m *Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'q' to exit", m.err)
	}
	if m.deploy == nil {
		return fmt.Sprintf("Deploying service %s...\n\nPress 'q' to exit", m.service.Name)
	}
	// todo: tail logs here instead of just showing the deploy info
	return fmt.Sprintf("Deploy ID: %s\nStatus: %s\n\nPress 'q' to exit", m.deploy.Id, *m.deploy.Status)
}

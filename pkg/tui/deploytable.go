package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/deploys"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type DeployTableModel struct {
	deployRepo *deploys.DeployRepo
	loading    bool
	table      table.Model
	spinner    spinner.Model
	serviceID  string
	SelectedID string
}

func NewDeployTableModel(deployRepo *deploys.DeployRepo, serviceID string) DeployTableModel {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return DeployTableModel{
		deployRepo: deployRepo,
		spinner:    spin,
		loading:    true,
		serviceID:  serviceID,
	}
}

func (m DeployTableModel) loadDeploys() tea.Msg {
	deps, err := m.deployRepo.ListDeploysForService(m.serviceID)
	if err != nil {
		return loadedDeploysErrorMsg(err)
	}
	return loadedDeploysMsg(deps)
}

type loadedDeploysMsg []*client.Deploy
type loadedDeploysErrorMsg error

func (m DeployTableModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadDeploys)
}

func (m DeployTableModel) setTableData(msg loadedDeploysMsg) DeployTableModel {
	columns := []table.Column{
		{Title: "ID", Width: 16},
		{Title: "Commit Message", Width: 40},
		{Title: "Created", Width: 30},
		{Title: "Status", Width: 15},
	}
	var rows []table.Row
	for _, d := range msg {
		rows = append(rows, table.Row{
			d.Id,
			*d.Commit.Message,
			d.CreatedAt.String(),
			string(*d.Status),
		})
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)
	m.table = t
	m.loading = false
	return m
}

func (m DeployTableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case loadedDeploysMsg:
		return m.setTableData(msg), nil
	case loadedDeploysErrorMsg:
		m.loading = false
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			m.SelectedID = m.table.SelectedRow()[0]
			return m, tea.Quit
		}
	}
	if m.loading {
		m.spinner, cmd = m.spinner.Update(msg)
	} else {
		m.table, cmd = m.table.Update(msg)
	}
	return m, cmd
}

func (m DeployTableModel) View() string {
	if m.loading {
		return fmt.Sprintf("\n\n   %s Loading deploys...\n\n", m.spinner.View())
	}
	return baseStyle.Render(m.table.View()) + "\n"
}

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/deploy"
	"github.com/renderinc/render-cli/pkg/services"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

// servicesCmd represents the services command
var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "A brief description of your command",
	RunE: func(cmd *cobra.Command, args []string) error {
		stack := tui.NewStack()
		renderServices(stack)
		p := tea.NewProgram(stack)
		_, err := p.Run()
		if err != nil {
			return fmt.Errorf("error running program: %v", err)
		}

		return nil
	},
}

func renderServices(stack *tui.StackModel) {
	serviceRepo := services.NewServiceRepo(http.DefaultClient, os.Getenv("RENDER_HOST"), os.Getenv("RENDER_API_KEY"))

	columns := []table.Column{
		{
			Title: "ID",
			Width: 25,
		},
		{
			Title: "Name",
			Width: 40,
		},
	}

	fmtFunc := func(a *client.Service) table.Row {
		return []string{a.Id, a.Name}
	}
	selectFunc := func(a *client.Service) tea.Cmd {
		return func() tea.Msg {
			return renderDeploys(a.Id)
		}
	}
	filterFunc := func(a *client.Service, filter string) bool {
		bytes, err := json.Marshal(a)
		if err != nil {
			return false
		}
		return strings.Contains(strings.ToLower(string(bytes)), filter)
	}

	m := tui.NewTableModel[*client.Service](
		"services",
		serviceRepo.ListServices,
		fmtFunc,
		selectFunc,
		columns,
		filterFunc,
		[]tui.CustomOption[*client.Service]{
			{
				Key:   "d",
				Title: "Deploy",
				Function: func(service *client.Service) tui.CustomAction {
					return &deploy.Action{
						Service: service,
						Repo:    serviceRepo,
					}
				},
			},
		},
	)
	stack.Push(m)
}

func init() {
	rootCmd.AddCommand(servicesCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// servicesCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// servicesCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

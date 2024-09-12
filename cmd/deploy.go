package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/deploys"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "A brief description of your command",
	RunE: func(cmd *cobra.Command, args []string) error {
		stack := tui.NewStack()

		serviceID := args[0]

		deployModel := renderDeploys(serviceID)
		stack.Push(deployModel)
		p := tea.NewProgram(stack)
		_, err := p.Run()
		if err != nil {
			return fmt.Errorf("error running program: %v", err)
		}

		return nil
	},
}

func renderDeploys(serviceID string) tea.Model {
	deployRepo := deploys.NewDeployRepo(http.DefaultClient, os.Getenv("RENDER_HOST"), os.Getenv("RENDER_API_KEY"))
	columns := []table.Column{
		{Title: "ID", Width: 25},
		{Title: "Commit Message", Width: 40},
		{Title: "Created", Width: 30},
		{Title: "Status", Width: 15},
	}

	fmtFunc := func(a *client.Deploy) table.Row {
		return []string{a.Id, refForDeploy(a), a.CreatedAt.String(), string(*a.Status)}
	}
	selectFunc := func(a *client.Deploy) tea.Cmd {
		return func() tea.Msg {
			return nil
		}
	}

	return tui.NewTableModel[*client.Deploy]("deploys", func() ([]*client.Deploy, error) { return deployRepo.ListDeploysForService(serviceID) }, fmtFunc, selectFunc, columns)
}

func refForDeploy(deploy *client.Deploy) string {
	if deploy.Commit != nil {
		return *deploy.Commit.Message
	}
	if deploy.Image != nil {
		return *deploy.Image.Ref
	}
	return ""
}

func init() {
	rootCmd.AddCommand(deployCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deployCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// deployCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

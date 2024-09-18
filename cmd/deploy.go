package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/deploy"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "A brief description of your command",
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceID := args[0]

		input := ListDeployInput{ServiceID: serviceID}

		command.Wrap(cmd, loadDeployData, renderDeploys)(cmd.Context(), input)

		return nil
	},
}

func loadDeployData(input ListDeployInput) ([]*client.Deploy, error) {
	deployRepo := deploy.NewDeployRepo(http.DefaultClient, os.Getenv("RENDER_HOST"), os.Getenv("RENDER_API_KEY"))
	return deployRepo.ListDeploysForService(input.ServiceID)
}

type ListDeployInput struct {
	ServiceID string
}

var InteractiveDeploys = command.Wrap(deployCmd, loadDeployData, renderDeploys)

func (l ListDeployInput) String() []string {
	return []string{l.ServiceID}
}

func renderDeploys(_ context.Context, loadData func() ([]*client.Deploy, error)) (tea.Model, error) {
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

	filterFunc := func(a *client.Deploy, filter string) bool {
		bytes, err := json.Marshal(a)
		if err != nil {
			return false
		}
		return strings.Contains(string(bytes), filter)
	}

	return tui.NewTableModel[*client.Deploy](
		"deploys",
		loadData,
		fmtFunc,
		selectFunc,
		columns,
		filterFunc,
		[]tui.CustomOption[*client.Deploy]{},
	), nil
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

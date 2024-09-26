package cmd

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/cfg"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/config"
	"github.com/renderinc/render-cli/pkg/owner"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Select a workspace to run commands against",
}

var InteractiveWorkspace = command.Wrap(workspaceCmd, loadWorkspaceData, renderWorkspaces)

func loadWorkspaceData(ctx context.Context, _ ListWorkspaceInput) ([]*client.Owner, error) {
	c, err := client.ClientWithAuth(http.DefaultClient, cfg.GetHost(), cfg.GetAPIKey())
	if err != nil {
		return nil, err
	}

	ownerRepo := owner.NewRepo(c)
	result, err := ownerRepo.ListOwners(ctx)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type ListWorkspaceInput struct{}

func (l ListWorkspaceInput) String() []string {
	return []string{}
}

func renderWorkspaces(
	ctx context.Context,
	loadData func(input ListWorkspaceInput) ([]*client.Owner, error),
	input ListWorkspaceInput,
) (tea.Model, error) {
	columns := []table.Column{
		{Title: "ID", Width: 36},
		{Title: "Name", Width: 30},
		{Title: "Email", Width: 30},
		{Title: "Type", Width: 15},
	}

	load := func() ([]*client.Owner, error) {
		return loadData(input)
	}

	return tui.NewTableModel[*client.Owner](
		"workspaces",
		load,
		formatWorkspaceRow,
		selectWorkspace,
		columns,
		filterWorkspace,
		[]tui.CustomOption[*client.Owner]{},
	), nil
}

func formatWorkspaceRow(o *client.Owner) table.Row {
	return []string{o.Id, o.Name, o.Email, string(o.Type)}
}

func selectWorkspace(o *client.Owner) tea.Cmd {
	return func() tea.Msg {
		conf, err := config.Load()
		if err != nil {
			return tui.ErrorMsg{Err: fmt.Errorf("failed to load config: %w", err)}
		}

		conf.Workspace = o.Id
		if err := conf.Persist(); err != nil {
			return tui.ErrorMsg{Err: fmt.Errorf("failed to persist config: %w", err)}
		}

		return tui.DoneMsg{Message: fmt.Sprintf("Workspace set to %s", o.Name)}
	}
}

func filterWorkspace(o *client.Owner, filter string) bool {
	return strings.Contains(strings.ToLower(o.Id), filter) ||
		strings.Contains(strings.ToLower(o.Name), filter) ||
		strings.Contains(strings.ToLower(o.Email), filter) ||
		strings.Contains(strings.ToLower(string(o.Type)), filter)
}

func init() {
	workspaceCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input ListWorkspaceInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}
		InteractiveWorkspace(cmd.Context(), input)
		return nil
	}

	rootCmd.AddCommand(workspaceCmd)
}

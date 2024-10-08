package cmd

import (
	"context"
	"fmt"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
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
	Long: `Select a workspace to run commands against.
Your specified workspace will be saved in a config file specified by the RENDER_CLI_CONFIG_PATH environment variable.
If unspecified, the config file will be saved in $HOME/.render/cli.yaml. All subsequent commands will run against this workspace.

Currently, you can only select a workspace in interactive mode.
`,
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

const columnWorkspaceIDKey = "ID"
const columnWorkspaceNameKey = "Name"
const columnWorkspaceEmailKey = "Email"

func renderWorkspaces(
	ctx context.Context,
	loadData func(input ListWorkspaceInput) ([]*client.Owner, error),
	input ListWorkspaceInput,
) (tea.Model, error) {
	columns := []btable.Column{
		btable.NewColumn(columnWorkspaceIDKey, "ID", 28),
		btable.NewFlexColumn(columnWorkspaceNameKey, "Name", 1),
		btable.NewFlexColumn(columnWorkspaceEmailKey, "Email", 1),
	}

	owners, err := loadData(input)
	if err != nil {
		return nil, err
	}

	var rows []btable.Row
	for _, o := range owners {
		rows = append(rows, btable.NewRow(btable.RowData{
			"ID":    o.Id,
			"Name":  o.Name,
			"Email": o.Email,
		}))
	}

	onSelect := func(data []btable.Row) tea.Cmd {
		return func() tea.Msg {
			if len(data) == 0 || len(data) > 1 {
				return nil
			}

			selectedID, ok := data[0].Data["ID"].(string)
			if !ok {
				return nil
			}

			for _, o := range owners {
				if o.Id == selectedID {
					return selectWorkspace(o)
				}
			}

			return nil
		}
	}

	t := tui.NewNewTable(
		columns,
		rows,
		onSelect,
	)

	return t, nil
}

func selectWorkspace(o *client.Owner) tea.Msg {
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

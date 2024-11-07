package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/config"
	"github.com/renderinc/render-cli/pkg/owner"
	"github.com/renderinc/render-cli/pkg/tui"
)

type ListWorkspaceInput struct{}

func selectWorkspace(o *client.Owner) tea.Msg {
	conf, err := config.Load()
	if err != nil {
		return tui.ErrorMsg{Err: fmt.Errorf("failed to load config: %w", err)}
	}

	conf.Workspace = o.Id
	conf.WorkspaceName = o.Name
	if err := conf.Persist(); err != nil {
		return tui.ErrorMsg{Err: fmt.Errorf("failed to persist config: %w", err)}
	}

	return tui.DoneMsg{Message: fmt.Sprintf("Workspace set to %s", o.Name)}
}

func loadWorkspaceData(ctx context.Context, _ ListWorkspaceInput) ([]*client.Owner, error) {
	c, err := client.NewDefaultClient()
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

type WorkspaceView struct {
	table *tui.Table[*client.Owner]
}

func NewWorkspaceView(ctx context.Context, input ListWorkspaceInput) *WorkspaceView {
	columns := []btable.Column{
		btable.NewFlexColumn("Name", "Name", 1).WithFiltered(true),
		btable.NewFlexColumn("Email", "Email", 1).WithFiltered(true),
		btable.NewColumn("ID", "ID", 28).WithFiltered(true),
	}

	createRowFunc := func(owner *client.Owner) btable.Row {
		return btable.NewRow(btable.RowData{
			"ID":    owner.Id,
			"Name":  owner.Name,
			"Email": owner.Email,
		})
	}

	onSelect := func(rows []btable.Row) tea.Cmd {
		return func() tea.Msg {
			if len(rows) == 0 {
				return nil
			}

			selectedID, ok := rows[0].Data["ID"].(string)
			if !ok {
				return nil
			}

			owners, err := loadWorkspaceData(ctx, input)
			if err != nil {
				return tui.ErrorMsg{Err: fmt.Errorf("failed to load owners: %w", err)}
			}

			for _, o := range owners {
				if o.Id == selectedID {
					if err := config.ClearProjectFilter(); err != nil {
						return tui.ErrorMsg{Err: fmt.Errorf("failed to clear project filter on workspace change: %w", err)}
					}
					return selectWorkspace(o)
				}
			}

			return nil
		}
	}

	t := tui.NewTable(
		columns,
		command.LoadCmd(ctx, loadWorkspaceData, input),
		createRowFunc,
		onSelect,
	)

	return &WorkspaceView{
		table: t,
	}
}

func (v *WorkspaceView) Init() tea.Cmd {
	return v.table.Init()
}

func (v *WorkspaceView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v.table.Update(msg)
}

func (v *WorkspaceView) View() string {
	return v.table.View()
}

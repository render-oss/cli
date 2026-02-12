package views

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/owner"
	"github.com/render-oss/cli/pkg/tui"
)

const teamIDPrefix = "tea-"
const userIDPrefix = "usr-"

type ListWorkspaceInput struct{}

type GetWorkspaceInput struct {
	IDOrName string
}

func SelectWorkspace(ctx context.Context, input GetWorkspaceInput) (*client.Owner, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	ownerRepo := owner.NewRepo(c)

	var own *client.Owner
	if strings.HasPrefix(input.IDOrName, teamIDPrefix) || strings.HasPrefix(input.IDOrName, userIDPrefix) {
		own, err = ownerRepo.RetrieveOwner(ctx, input.IDOrName)
		if err != nil {
			return nil, err
		}
	} else {
		owners, err := ownerRepo.ListOwners(ctx, owner.ListInput{Name: input.IDOrName})
		if err != nil {
			return nil, err
		}
		if len(owners) == 0 {
			return nil, fmt.Errorf("no workspaces found with name %s", input.IDOrName)
		}

		if len(owners) > 1 {
			return nil, fmt.Errorf("multiple workspaces found with name %s; please specify workspace id", input.IDOrName)
		}
		own = owners[0]
	}

	_, err = selectWorkspace(own)
	if err != nil {
		return nil, err
	}
	return own, nil
}

func selectWorkspace(o *client.Owner) (string, error) {
	conf, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	conf.Workspace = o.Id
	conf.WorkspaceName = o.Name
	if err := conf.Persist(); err != nil {
		return "", fmt.Errorf("failed to persist config: %w", err)
	}

	return fmt.Sprintf("Workspace set to %s", o.Name), nil
}

func loadWorkspaceData(ctx context.Context, _ ListWorkspaceInput) ([]*client.Owner, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	ownerRepo := owner.NewRepo(c)
	result, err := ownerRepo.ListOwners(ctx, owner.ListInput{})
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
					msg, err := selectWorkspace(o)
					if err != nil {
						return tui.ErrorMsg{Err: err}
					}
					return tui.DoneMsg{Message: msg}
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
	model, cmd := v.table.Update(msg)

	if _, ok := msg.(tui.LoadDataMsg[[]*client.Owner]); ok {
		currentID, _ := config.WorkspaceID()
		for i, row := range v.table.Model.GetVisibleRows() {
			if id, _ := row.Data["ID"].(string); id == currentID {
				v.table.Model = v.table.Model.WithHighlightedRow(i)
				break
			}
		}
	}

	return model, cmd
}

func (v *WorkspaceView) View() string {
	return v.table.View()
}

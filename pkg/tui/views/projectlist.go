package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/project"
	"github.com/renderinc/render-cli/pkg/tui"
)

type ProjectList struct {
	table *tui.Table[*client.Project]
}

type OnSelectFunc func(context.Context, *client.Project) tea.Cmd

func NewProjectList(ctx context.Context, selectEnvironment OnSelectFunc, opts ...tui.TableOption[*client.Project]) *ProjectList {
	columns := []btable.Column{
		btable.NewFlexColumn("Name", "Name", 4).WithFiltered(true),
		btable.NewColumn("ID", "ID", 25).WithFiltered(true),
	}

	createRowFunc := func(p *client.Project) btable.Row {
		return btable.NewRow(btable.RowData{
			"ID":      p.Id,
			"Name":    p.Name,
			"project": p, // this will be hidden in the UI, but will be used to get the project when selected
		})
	}

	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		p, ok := rows[0].Data["project"].(*client.Project)
		if !ok {
			return nil
		}

		return selectEnvironment(ctx, p)
	}

	t := tui.NewTable(
		columns,
		command.LoadCmd(ctx, LoadProjects, ProjectInput{}),
		createRowFunc,
		onSelect,
		opts...,
	)

	return &ProjectList{
		table: t,
	}
}

type ProjectInput struct{}

func LoadProjects(ctx context.Context, _ ProjectInput) ([]*client.Project, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	projectRepo := project.NewRepo(c)
	return projectRepo.ListProjects(ctx)
}

func (pl *ProjectList) Init() tea.Cmd {
	return pl.table.Init()
}

func (pl *ProjectList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return pl.table.Update(msg)
}

func (pl *ProjectList) View() string {
	return pl.table.View()
}

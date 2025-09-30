package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/resource"
	resourcetui "github.com/render-oss/cli/pkg/resource/tui"
	"github.com/render-oss/cli/pkg/tui"
)

type ResourceView struct {
	table *tui.Table[resource.Resource]
}

func NewResourceView(ctx context.Context,
	input ListResourceInput,
	loadResourceData func(ctx context.Context, in ListResourceInput) ([]resource.Resource, error),
	onSelect func(r resource.Resource) tea.Cmd,
	opts ...tui.TableOption[resource.Resource],
) *ResourceView {
	resourceView := &ResourceView{}

	onSelectWrapper := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		r, ok := rows[0].Data["resource"].(resource.Resource)
		if !ok {
			return nil
		}

		return onSelect(r)
	}

	// check for a persistent project filter if other input has not been provided
	if len(input.EnvironmentIDs) == 0 {
		savedInput, err := DefaultListResourceInput(ctx)
		if err == nil && savedInput.Project != nil {
			input.Project = savedInput.Project
			input.EnvironmentIDs = savedInput.EnvironmentIDs
		}
	}

	if input.Project != nil {
		opts = append(opts, tui.WithHeader[resource.Resource](
			fmt.Sprintf("Project: %s", input.Project.Name),
		))
	}

	resourceView.table = tui.NewTable(
		resourcetui.ColumnsForResources(),
		command.LoadCmd(ctx, loadResourceData, input),
		resourcetui.RowForResource,
		onSelectWrapper,
		opts...,
	)

	return resourceView
}

func (v *ResourceView) Init() tea.Cmd {
	return v.table.Init()
}

func (v *ResourceView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := v.table.Update(msg)
	return v, cmd
}

func (v *ResourceView) View() string {
	return v.table.View()
}

func DefaultListResourceInput(ctx context.Context) (ListResourceInput, error) {
	projectID, _, err := config.GetProjectFilter()
	if err != nil {
		return ListResourceInput{}, err
	}

	if projectID == "" {
		return ListResourceInput{}, nil
	}

	c, err := client.NewDefaultClient()
	if err != nil {
		return ListResourceInput{}, err
	}

	projectRepo := project.NewRepo(c)
	p, err := projectRepo.GetProject(ctx, projectID)
	if err != nil {
		return ListResourceInput{}, err
	}

	return ListResourceInput{
		Project:        p,
		EnvironmentIDs: p.EnvironmentIds,
	}, nil
}

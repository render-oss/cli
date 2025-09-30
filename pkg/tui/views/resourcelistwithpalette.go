package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/render-oss/cli/pkg/resource"
	"github.com/render-oss/cli/pkg/tui"
)

type ResourceWithPaletteView struct {
	resourceList *ResourceView
	palette      *PaletteView
}

func NewResourceWithPaletteView(ctx context.Context, input ListResourceInput, loadResourceData func(ctx context.Context, in ListResourceInput) ([]resource.Resource, error), commandsForResource func(r resource.Resource) tea.Cmd, opts ...tui.TableOption[resource.Resource]) *ResourceWithPaletteView {
	resourceView := &ResourceWithPaletteView{}
	resourceView.resourceList = NewResourceView(ctx, input, loadResourceData, commandsForResource, opts...)
	return resourceView
}

func (v *ResourceWithPaletteView) Init() tea.Cmd {
	if v.palette != nil {
		return v.palette.Init()
	}
	return v.resourceList.Init()
}

func (v *ResourceWithPaletteView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if v.palette != nil {
		_, cmd := v.palette.Update(msg)
		return v, cmd
	}
	_, cmd := v.resourceList.Update(msg)
	return v, cmd
}

func (v *ResourceWithPaletteView) View() string {
	if v.palette != nil {
		return v.palette.View()
	}
	return v.resourceList.View()
}

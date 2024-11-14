package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/renderinc/cli/pkg/config"
)

type ConfigWrapper struct {
	stack         *StackModel
	configured    bool
	workspaceView tea.Model
}

func NewConfigWrapper(stack *StackModel, view tea.Model) *ConfigWrapper {
	return &ConfigWrapper{
		stack:         stack,
		workspaceView: view,
	}
}

func (c *ConfigWrapper) Init() tea.Cmd {
	workspace, err := config.WorkspaceID()
	if err == nil && workspace != "" {
		c.configured = true
		return c.stack.Init()
	}

	return c.workspaceView.Init()
}

func (c *ConfigWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if c.configured {
		return c.stack, c.stack.Init()
	}

	switch msg.(type) {
	case DoneMsg:
		c.configured = true
		return c.stack, c.stack.Init()
	}

	return c.workspaceView.Update(msg)
}

func (c *ConfigWrapper) View() string {
	if c.configured {
		return c.stack.View()
	}

	return c.workspaceView.View()
}

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type ConfigWrapper struct {
	next       tea.Model
	configView tea.Model

	configured bool
}

func NewConfigWrapper(next tea.Model, breadcrumb string, m tea.Model) *ConfigWrapper {
	stack := NewStack()
	stack.Push(ModelWithCmd{
		Model:      m,
		Breadcrumb: breadcrumb,
	})

	w := &ConfigWrapper{
		next:       next,
		configView: stack,
	}
	stack.WithDone(w.Update)

	return w
}

func (c *ConfigWrapper) Init() tea.Cmd {
	cmd := c.configView.Init()
	if cmd != nil {
		return cmd
	}

	c.configured = true
	return c.next.Init()
}

func (c *ConfigWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if c.configured {
		return c.next, c.next.Init()
	}

	switch msg.(type) {
	case DoneMsg:
		c.configured = true
		return c.next, c.next.Init()
	}

	return c.configView.Update(msg)
}

func (c *ConfigWrapper) View() string {
	if c.configured {
		return c.next.View()
	}

	return c.configView.View()
}

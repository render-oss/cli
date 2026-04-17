package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type ConfigWrapper struct {
	next       tea.Model
	configView tea.Model

	configured bool
	lastSize   *tea.WindowSizeMsg
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
		return c.next, nil
	}

	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		size := wsm
		c.lastSize = &size
	}

	switch msg.(type) {
	case DoneMsg:
		c.configured = true
		// Replay the last-seen WindowSizeMsg so c.next learns the terminal
		// dimensions. Without this, any downstream size-aware model
		// (forms, tables, stacks) renders at zero dimensions until the
		// user resizes.
		cmds := []tea.Cmd{c.next.Init()}
		if c.lastSize != nil {
			size := *c.lastSize
			cmds = append(cmds, func() tea.Msg { return size })
		}
		return c.next, tea.Batch(cmds...)
	}

	// Stay as root while unconfigured. If we returned c.configView here,
	// bubbletea would adopt it and later WindowSizeMsgs would never reach
	// us to be captured — so when DoneMsg arrives we'd have nothing to
	// replay.
	_, cmd := c.configView.Update(msg)
	return c, cmd
}

func (c *ConfigWrapper) View() string {
	if c.configured {
		return c.next.View()
	}

	return c.configView.View()
}

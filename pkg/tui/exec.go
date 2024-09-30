package tui

import (
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

func NewExecModel(cmd *exec.Cmd) *ExecModel {
	return &ExecModel{
		cmd: cmd,
	}
}

type ExecModel struct {
	cmd *exec.Cmd
}

type ExecDone struct{}

func (m *ExecModel) Init() tea.Cmd {
	return tea.ExecProcess(m.cmd, func(_ error) tea.Msg {
		return ExecDone{}
	})
}

func (m *ExecModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(ExecDone); ok {
		return m, func() tea.Msg { return DoneMsg{Message: "Done"} }
	}

	return m, nil
}

func (m *ExecModel) View() string {
	return ""
}

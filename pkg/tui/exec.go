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

type ExecDone struct{
	Error error
}

func (m *ExecModel) Init() tea.Cmd {
	return tea.ExecProcess(m.cmd, func(err error) tea.Msg {
		return ExecDone{
			Error: err,
		}
	})
}

func (m *ExecModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if execMsg, ok := msg.(ExecDone); ok {
		return m, func() tea.Msg { 
			if execMsg.Error != nil {
				return ErrorMsg{
					Err: execMsg.Error,
				}
			}
			return DoneMsg{Message: "Done"} 
		}
	}

	return m, nil
}

func (m *ExecModel) View() string {
	return ""
}

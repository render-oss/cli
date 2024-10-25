package tui

import (
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

func NewExecModel(loadCmd TypedCmd[*exec.Cmd]) *ExecModel {
	return &ExecModel{
		loadCmd: loadCmd,
	}
}

type ExecModel struct {
	loadCmd TypedCmd[*exec.Cmd]
}

type ExecDone struct {
	Error error
}

func (m *ExecModel) Init() tea.Cmd {
	return m.loadCmd.Unwrap()
}

func (m *ExecModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LoadDataMsg[*exec.Cmd]:
		return m, tea.ExecProcess(msg.Data, func(err error) tea.Msg {
			return ExecDone{
				Error: err,
			}
		})
	case ExecDone:
		return m, func() tea.Msg {
			if msg.Error != nil {
				return ErrorMsg{
					Err: msg.Error,
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

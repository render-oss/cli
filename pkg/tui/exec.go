package tui

import (
	"errors"
	"fmt"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

func NewExecModel(executableName string, handleError func(err error) error, loadCmd TypedCmd[*exec.Cmd]) *ExecModel {
	return &ExecModel{
		executableName: executableName,
		handleError:    handleError,
		loadCmd:        loadCmd,
	}
}

type ExecModel struct {
	executableName string
	handleError    func(err error) error
	loadCmd        TypedCmd[*exec.Cmd]
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
			// We have a number of precondition checks to ensure that we can provide a user-friendly error message. If
			// it's a user-facing error, just return it as-is.
			if errors.As(err, &UserFacingError{}) {
				return ExecDone{
					Error: err,
				}
			}

			if errors.Is(err, exec.ErrNotFound) {
				return ExecDone{
					Error: UserFacingError{
						Title:   fmt.Sprintf("%s not found on path", m.executableName),
						Message: fmt.Sprintf("Please ensure %s is installed and try again", m.executableName),
					},
				}
			}

			// This error occurred when running the command. Wrap it in a user-facing error with a helpful message.
			if err != nil {
				return ExecDone{
					Error: m.handleError(err),
				}
			}

			return ExecDone{}
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

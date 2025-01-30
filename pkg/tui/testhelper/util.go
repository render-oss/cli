package testhelper

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/render-oss/cli/pkg/tui"
)

func Stackify(m tea.Model) tea.Model {
	stack := tui.NewStack()
	stack.Push(tui.ModelWithCmd{Model: m, Cmd: ""})
	return stack
}

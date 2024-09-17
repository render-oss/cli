package command

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

type Arguments interface {
	String() []string
}

type WrappedFunc[T Arguments] func(stack *tui.StackModel, args T) tea.Cmd

func Wrap[T Arguments](cmd *cobra.Command, fn func(*tui.StackModel, T) (tea.Model, error)) WrappedFunc[T] {
	return func(stack *tui.StackModel, args T) tea.Cmd {
		model, err := fn(stack, args)
		if err != nil {
			return tea.Quit
		}

		stack.Push(tui.ModelWithCmd{
			Model: model, Cmd: CommandName(cmd, args.String(), nil),
		})

		return model.Init()
	}

}

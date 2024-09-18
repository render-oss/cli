package command

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

type Arguments interface {
	String() []string
}

type WrappedFunc[T Arguments] func(ctx context.Context, args T) tea.Cmd

func Wrap[T Arguments](cmd *cobra.Command, fn func(context.Context, T) (tea.Model, error)) WrappedFunc[T] {
	return func(ctx context.Context, args T) tea.Cmd {
		stack := tui.GetStackFromContext(ctx)
		model, err := fn(ctx, args)
		if err != nil {
			return tea.Quit
		}

		stack.Push(tui.ModelWithCmd{
			Model: model, Cmd: CommandName(cmd, args.String(), nil),
		})

		return model.Init()
	}

}

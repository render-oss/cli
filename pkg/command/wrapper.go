package command

import (
	"context"
	"encoding/json"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Arguments interface {
	String() []string
}

type WrappedFunc[T Arguments] func(ctx context.Context, args T) tea.Cmd

type InteractiveFunc[T Arguments, D any] func(context.Context, func(T) (D, error), T) (tea.Model, error)

func Wrap[T Arguments, D any](cmd *cobra.Command, loadData func(context.Context, T) (D, error), interactiveFunc InteractiveFunc[T, D]) WrappedFunc[T] {
	return func(ctx context.Context, args T) tea.Cmd {
		outputFormat := GetFormatFromContext(ctx)

		if outputFormat != nil && (*outputFormat == JSON || *outputFormat == YAML) {
			data, err := loadData(ctx, args)
			if err != nil {
				_, _ = cmd.ErrOrStderr().Write([]byte(err.Error()))
				return nil
			}

			switch *outputFormat {
			case JSON:
				jsonStr, err := json.MarshalIndent(data, "", "  ")
				if err != nil {
					return nil
				}
				if _, err := cmd.OutOrStdout().Write(jsonStr); err != nil {
					return nil
				}
			case YAML:
				yamlStr, err := yaml.Marshal(data)
				if err != nil {
					return nil
				}
				if _, err := cmd.OutOrStdout().Write(yamlStr); err != nil {
					return nil
				}
			}

			return nil
		}

		stack := tui.GetStackFromContext(ctx)
		model, err := interactiveFunc(ctx, func(T) (D, error) { return loadData(ctx, args) }, args)
		if err != nil {
			errModel := tui.NewErrorModel(err.Error())
			stack.Push(tui.ModelWithCmd{
				Model: errModel, Cmd: CommandName(cmd, args.String(), nil),
			})
			_, _ = cmd.ErrOrStderr().Write([]byte(err.Error()))
			return func() tea.Msg { return tui.ErrorMsg{Err: err} }
		}

		stack.Push(tui.ModelWithCmd{
			Model: model, Cmd: CommandName(cmd, args.String(), nil),
		})

		return model.Init()
	}

}

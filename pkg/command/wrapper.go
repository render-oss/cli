package command

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const ConfirmFlag = "confirm"

type WrappedFunc[T any] func(ctx context.Context, args T) tea.Cmd

type InteractiveFunc[T any, D any] func(context.Context, func(T) (D, error), T) (tea.Model, error)

type RequireConfirm[T any] struct {
	Confirm     bool
	MessageFunc func(args T) string
}

type WrapOptions[T any] struct {
	RequireConfirm RequireConfirm[T]
}

func nonInteractive[T any, D any](ctx context.Context, outputFormat *Output, cmd *cobra.Command, loadData func(context.Context, T) (D, error), args T, opts *WrapOptions[T]) error {
	if opts != nil && opts.RequireConfirm.Confirm {
		if confirm := GetConfirmFromContext(ctx); !confirm {
			msg := opts.RequireConfirm.MessageFunc(args)
			_, err := cmd.OutOrStdout().Write([]byte(fmt.Sprintf("%s (y/n): ", msg)))
			if err != nil {
				return err
			}

			reader := bufio.NewReader(cmd.InOrStdin())
			str, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			if str != "y\n" {
				_, err := cmd.OutOrStdout().Write([]byte("Aborted\n"))
				return err
			}
		}
	}

	data, err := loadData(ctx, args)
	if err != nil {
		return err
	}

	switch *outputFormat {
	case JSON:
		jsonStr, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		if _, err := cmd.OutOrStdout().Write(jsonStr); err != nil {
			return err
		}
	case YAML:
		yamlStr, err := yaml.Marshal(data)
		if err != nil {
			return err
		}
		if _, err := cmd.OutOrStdout().Write(yamlStr); err != nil {
			return err
		}
	}

	return nil
}

func Wrap[T any, D any](cmd *cobra.Command, loadData func(context.Context, T) (D, error), interactiveFunc InteractiveFunc[T, D], opts *WrapOptions[T]) WrappedFunc[T] {
	return func(ctx context.Context, args T) tea.Cmd {
		outputFormat := GetFormatFromContext(ctx)

		if outputFormat != nil && (*outputFormat == JSON || *outputFormat == YAML) {
			if err := nonInteractive(ctx, outputFormat, cmd, loadData, args, opts); err != nil {
				_, _ = cmd.ErrOrStderr().Write([]byte(err.Error()))
				return nil
			} else {
				return nil
			}
		}

		var cmdString string
		if !cmd.Hidden {
			var err error
			cmdString, err = CommandName(cmd, &args)
			if err != nil {
				return func() tea.Msg { return tui.ErrorMsg{Err: err} }
			}
		}

		stack := tui.GetStackFromContext(ctx)
		model, err := interactiveFunc(ctx, func(T) (D, error) { return loadData(ctx, args) }, args)
		if err != nil {
			errModel := tui.NewErrorModel(err.Error())
			stack.Push(tui.ModelWithCmd{
				Model: errModel, Cmd: cmdString,
			})
			_, _ = cmd.ErrOrStderr().Write([]byte(err.Error()))
			return func() tea.Msg { return tui.ErrorMsg{Err: err} }
		}

		confirm := GetConfirmFromContext(ctx)
		if opts != nil && opts.RequireConfirm.Confirm && !confirm {
			model = tui.NewModelWithConfirm(model, opts.RequireConfirm.MessageFunc(args))
		}

		stack.Push(tui.ModelWithCmd{
			Model: model, Cmd: cmdString,
		})

		return model.Init()
	}
}

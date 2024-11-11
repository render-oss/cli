package command

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/tui"
)

const ConfirmFlag = "confirm"

var ErrTokenExpired = errors.New("your token is expired; run `render login` to get a new one")

type WrappedFunc[T any] func(ctx context.Context, args T) tea.Cmd

type InteractiveFunc[T any, D any] func(context.Context, func(T) tui.TypedCmd[D], T) (tea.Model, error)

type RequireConfirm[T any] struct {
	Confirm     bool
	MessageFunc func(ctx context.Context, args T) (string, error)
}

type WrapOptions[T any] struct {
	RequireConfirm RequireConfirm[T]
}

func NonInteractive(cmd *cobra.Command, loadData func() (any, error), confirmMessageFunc func() (string, error)) (bool, error) {
	outputFormat := GetFormatFromContext(cmd.Context())

	if outputFormat == nil || !(*outputFormat == JSON || *outputFormat == YAML) {
		return false, nil
	}

	if confirmMessageFunc != nil {
		if confirm := GetConfirmFromContext(cmd.Context()); !confirm {
			message, err := confirmMessageFunc()
			if err != nil {
				return false, err
			}
			_, err = cmd.OutOrStdout().Write([]byte(fmt.Sprintf("%s (y/n): ", message)))
			if err != nil {
				return false, err
			}

			reader := bufio.NewReader(cmd.InOrStdin())
			str, err := reader.ReadString('\n')
			if err != nil {
				return false, err
			}
			if str != "y\n" {
				_, err := cmd.OutOrStdout().Write([]byte("Aborted\n"))
				return false, err
			}
		}
	}

	data, err := loadData()
	if errors.Is(err, client.ErrUnauthorized) {
		return false, ErrTokenExpired
	}
	if err != nil {
		return false, err
	}

	return PrintData(cmd, data)
}

func PrintData(cmd *cobra.Command, data any) (bool, error) {
	outputFormat := GetFormatFromContext(cmd.Context())

	switch *outputFormat {
	case JSON:
		jsonStr, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return true, err
		}
		_, err = cmd.OutOrStdout().Write(jsonStr)
		return true, err
	case YAML:
		yamlStr, err := yaml.Marshal(data)
		if err != nil {
			return true, err
		}
		_, err = cmd.OutOrStdout().Write(yamlStr)
		return true, err
	}
	return false, nil
}

func wrappedModel(model tea.Model, cmd *cobra.Command, breadcrumb string, in any) (*tui.ModelWithCmd, error) {
	var cmdString string

	if !cmd.Hidden {
		var err error
		cmdString, err = CommandName(cmd, in)
		if err != nil {
			return nil, err
		}
	}

	confirmModel := tui.NewModelWithConfirm(model)

	return &tui.ModelWithCmd{
		Model:      confirmModel,
		Cmd:        cmdString,
		Breadcrumb: breadcrumb,
	}, nil
}

func AddToStackFunc[T any](ctx context.Context, cmd *cobra.Command, breadcrumb string, in T, m tea.Model) tea.Cmd {
	modelWithCmd, err := wrappedModel(m, cmd, breadcrumb, in)
	if err != nil {
		return nil
	}

	stack := tui.GetStackFromContext(ctx)
	return stack.Push(*modelWithCmd)

}

func LoadCmd[T any, D any](ctx context.Context, loadData func(context.Context, T) (D, error), in T) tui.TypedCmd[D] {
	loadDataCmd := func() tea.Msg {
		return tui.LoadingDataMsg(tea.Sequence(
			func() tea.Msg {
				data, err := loadData(ctx, in)
				if errors.Is(err, client.ErrUnauthorized) {
					return tui.ErrorMsg{Err: ErrTokenExpired}
				}
				if err != nil {
					return tui.ErrorMsg{Err: err}
				}
				return tui.LoadDataMsg[D]{Data: data}

			},
			func() tea.Msg {
				return tui.DoneLoadingDataMsg{}
			},
		))
	}
	return loadDataCmd
}

func WrapInConfirm[D any](cmd tui.TypedCmd[D], msgFunc func() (string, error)) tui.TypedCmd[D] {
	return func() tea.Msg {
		strMessage, err := msgFunc()
		if err != nil {
			return tui.ErrorMsg{Err: err}
		}

		return tui.ShowConfirmMsg{
			Message:   strMessage,
			OnConfirm: func() tea.Cmd { return cmd.Unwrap() },
		}
	}
}

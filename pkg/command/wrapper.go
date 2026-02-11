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

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/tui"
)

const ConfirmFlag = "confirm"

var ErrTokenExpired = errors.New("your token is expired; run `render login` to get a new one")
var ErrActionNotAllowed = errors.New("you are not allowed to take this action")

type WrappedFunc[T any] func(ctx context.Context, args T) tea.Cmd

type InteractiveFunc[T any, D any] func(context.Context, func(T) tui.TypedCmd[D], T) (tea.Model, error)

type RequireConfirm[T any] struct {
	Confirm     bool
	MessageFunc func(ctx context.Context, args T) (string, error)
}

type WrapOptions[T any] struct {
	RequireConfirm RequireConfirm[T]
}

type LoadDataFunc[T any] func() (T, error)
type FormatTextFunc[T any] func(T) string
type ConfirmFunc func() (string, error)

func NonInteractive[T any](cmd *cobra.Command, loadData LoadDataFunc[T], formatText FormatTextFunc[T]) (bool, error) {
	return NonInteractiveWithConfirm(cmd, loadData, formatText, nil)
}

func NonInteractiveWithConfirm[T any](cmd *cobra.Command, loadData LoadDataFunc[T], formatText FormatTextFunc[T], confirmMessageFunc ConfirmFunc) (bool, error) {
	outputFormat := GetFormatFromContext(cmd.Context())

	if outputFormat == nil || (*outputFormat == Interactive) {
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
	if err != nil {
		return false, convertToUserFacingErr(err)
	}

	return PrintData(cmd, data, formatText)
}

type TextTable interface {
	Header() []string
	Row() []string
}

func PrintData[T any](cmd *cobra.Command, data T, formatText FormatTextFunc[T]) (bool, error) {
	outputFormat := GetFormatFromContext(cmd.Context())

	switch *outputFormat {
	case JSON:
		return true, printJSON(cmd, data)
	case YAML:
		// Convert to JSON before converting to YAML to remove the top-level key of the containing struct and
		// null values that have omit_empty json tags. This is for consistency between JSON and YAML output.
		jsonStr, err := json.Marshal(data)
		if err != nil {
			return true, err
		}

		var yamlData interface{}
		err = json.Unmarshal(jsonStr, &yamlData)
		if err != nil {
			return true, err
		}

		yamlStr, err := yaml.Marshal(yamlData)
		if err != nil {
			return true, err
		}
		_, err = cmd.OutOrStdout().Write(yamlStr)
		return true, err
	case TEXT:
		_, err := cmd.OutOrStdout().Write([]byte(formatText(data)))
		return true, err
	}
	return false, nil
}

func printJSON(cmd *cobra.Command, data any) error {
	jsonStr, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write(jsonStr)
	return err
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

func AddErrToStack(ctx context.Context, cmd *cobra.Command, err error) tea.Cmd {
	if err == nil {
		return nil
	}

	return AddToStackFunc(ctx, cmd, "", err, tui.NewErrorModel(err))
}

func AddToStackFunc[T any](ctx context.Context, cmd *cobra.Command, breadcrumb string, in T, m tea.Model) tea.Cmd {
	stack := tui.GetStackFromContext(ctx)
	return AddToStack(stack, cmd, breadcrumb, in, m)
}

func AddToStack[T any](stack *tui.StackModel, cmd *cobra.Command, breadcrumb string, in T, m tea.Model) tea.Cmd {
	modelWithCmd, err := wrappedModel(m, cmd, breadcrumb, in)
	if err != nil {
		return nil
	}

	return stack.Push(*modelWithCmd)
}

func LoadCmd[T any, D any](ctx context.Context, loadData func(context.Context, T) (D, error), in T) tui.TypedCmd[D] {
	return LoadCmdWithLoadingMsg(ctx, loadData, in, "")
}

func LoadCmdWithLoadingMsg[T any, D any](ctx context.Context, loadData func(context.Context, T) (D, error), in T, loadingMsg string) tui.TypedCmd[D] {
	loadDataCmd := func() tea.Msg {
		return tui.LoadingDataMsg{
			LoadingMsgTmpl: loadingMsg,
			Cmd: tea.Sequence(
				func() tea.Msg {
					data, err := loadData(ctx, in)
					if err != nil {
						return tui.ErrorMsg{Err: convertToUserFacingErr(err)}
					}
					return tui.LoadDataMsg[D]{Data: data}
				},
				func() tea.Msg {
					return tui.DoneLoadingDataMsg{}
				},
			),
		}
	}
	return loadDataCmd
}

func PaginatedLoadCmd[T any, D any](ctx context.Context, loadData func(context.Context, T, client.Cursor) (client.Cursor, D, error), in T) tui.TypedCmd[D] {
	cursor := ""
	loadDataCmd := func() tea.Msg {
		return tui.LoadingDataMsg{
			Cmd: tea.Sequence(
				func() tea.Msg {
					next, data, err := loadData(ctx, in, cursor)
					if err != nil {
						return tui.ErrorMsg{Err: convertToUserFacingErr(err)}
					}
					cursor = next
					return tui.LoadDataMsg[D]{Data: data, HasMore: cursor != ""}
				},
				func() tea.Msg {
					return tui.DoneLoadingDataMsg{}
				},
			),
		}
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

func convertToUserFacingErr(err error) error {
	if errors.Is(err, client.ErrUnauthorized) {
		return ErrTokenExpired
	}

	if errors.Is(err, client.ErrForbidden) {
		return ErrActionNotAllowed
	}

	return err
}

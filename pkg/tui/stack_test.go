package tui_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/testhelper"
	"github.com/stretchr/testify/require"
)

func TestStack(t *testing.T) {
	t.Run("Can append and go back", func(t *testing.T) {
		stack := tui.NewStack()

		fooModel := &testhelper.SimpleModel{Str: "foo"}
		barModel := &testhelper.SimpleModel{Str: "bar"}

		stack.Push(tui.ModelWithCmd{Model: fooModel})
		tm := teatest.NewTestModel(t, stack)

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("foo"))
		}, teatest.WithCheckInterval(time.Millisecond*1), teatest.WithDuration(time.Second*3))

		tm.Send(stack.Push(tui.ModelWithCmd{Model: barModel})())

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return !bytes.Contains(bts, []byte("foo")) && bytes.Contains(bts, []byte("bar"))
		}, teatest.WithCheckInterval(time.Millisecond*1), teatest.WithDuration(time.Second*3))

		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlD})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("foo")) && !bytes.Contains(bts, []byte("bar"))
		}, teatest.WithCheckInterval(time.Millisecond*1), teatest.WithDuration(time.Second*3))

		err := tm.Quit()
		require.NoError(t, err)
	})

	t.Run("Popping the last item quits", func(t *testing.T) {
		stack := tui.NewStack()

		fooModel := &testhelper.SimpleModel{Str: "foo"}

		stack.Push(tui.ModelWithCmd{Model: fooModel})
		tm := teatest.NewTestModel(t, stack)

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("foo"))
		}, teatest.WithCheckInterval(time.Millisecond*1), teatest.WithDuration(time.Second*3))

		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlD})

		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*1))
	})

	t.Run("ctrl + c quits", func(t *testing.T) {
		stack := tui.NewStack()

		fooModel := &testhelper.SimpleModel{Str: "foo"}

		stack.Push(tui.ModelWithCmd{Model: fooModel})
		tm := teatest.NewTestModel(t, stack)

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*1))
	})

	t.Run("displays error", func(t *testing.T) {
		stack := tui.NewStack()

		fooModel := &testhelper.SimpleModel{Str: "foo"}

		stack.Push(tui.ModelWithCmd{Model: fooModel})
		tm := teatest.NewTestModel(t, stack)

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		tm.Send(tui.ErrorMsg{Err: fmt.Errorf("oh no")})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("oh no"))
		}, teatest.WithCheckInterval(time.Millisecond*1), teatest.WithDuration(time.Second*3))

		err := tm.Quit()
		require.NoError(t, err)
	})

	t.Run("displays loading spinner", func(t *testing.T) {
		stack := tui.NewStack()

		fooModel := &testhelper.SimpleModel{Str: "foo"}

		stack.Push(tui.ModelWithCmd{Model: fooModel})
		tm := teatest.NewTestModel(t, stack)

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		ch := make(chan interface{})

		cmd := command.LoadCmd(context.Background(), func(_ context.Context, _ any) (string, error) {
			_ = <-ch

			return "", nil
		}, nil).Unwrap()

		tm.Send(cmd())

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Loading"))
		}, teatest.WithCheckInterval(time.Millisecond*1), teatest.WithDuration(time.Second*3))

		close(ch)

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return !bytes.Contains(bts, []byte("Loading"))
		})

		err := tm.Quit()
		require.NoError(t, err)
	})
}

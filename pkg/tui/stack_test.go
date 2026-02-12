package tui_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/testhelper"
	"github.com/stretchr/testify/require"
)

func TestStackIsEmpty(t *testing.T) {
	t.Run("returns true for new stack", func(t *testing.T) {
		stack := tui.NewStack()
		require.True(t, stack.IsEmpty())
	})

	t.Run("returns false after push", func(t *testing.T) {
		stack := tui.NewStack()
		fooModel := &testhelper.SimpleModel{Str: "foo"}
		stack.Push(tui.ModelWithCmd{Model: fooModel})
		require.False(t, stack.IsEmpty())
	})

	t.Run("returns true after popping all items", func(t *testing.T) {
		stack := tui.NewStack()
		fooModel := &testhelper.SimpleModel{Str: "foo"}
		stack.Push(tui.ModelWithCmd{Model: fooModel})
		stack.Pop()
		require.True(t, stack.IsEmpty())
	})
}

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

func setupTestConfig(t *testing.T, content string) {
	t.Helper()
	f, err := os.CreateTemp("", "render-config")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	t.Setenv("RENDER_CLI_CONFIG_PATH", f.Name())
}

func TestStackHeader(t *testing.T) {
	t.Run("shows workspace name from config when no override set", func(t *testing.T) {
		setupTestConfig(t, "version: 1\nworkspace: tea-123\nworkspace_name: MyTeam\n")

		stack := tui.NewStack()
		stack.Push(tui.ModelWithCmd{Model: &testhelper.SimpleModel{Str: "content"}, Breadcrumb: "Services"})
		tm := teatest.NewTestModel(t, stack)

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("MyTeam")) && bytes.Contains(bts, []byte("Services"))
		}, teatest.WithCheckInterval(time.Millisecond*1), teatest.WithDuration(time.Second*3))

		err := tm.Quit()
		require.NoError(t, err)
	})

	t.Run("shows workspace override instead of config workspace name", func(t *testing.T) {
		setupTestConfig(t, "version: 1\nworkspace: tea-123\nworkspace_name: MyTeam\n")

		stack := tui.NewStack()
		stack.SetWorkspaceOverride("user@example.com")
		stack.Push(tui.ModelWithCmd{Model: &testhelper.SimpleModel{Str: "content"}, Breadcrumb: "Workspaces"})
		tm := teatest.NewTestModel(t, stack)

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("user@example.com")) &&
				bytes.Contains(bts, []byte("Workspaces")) &&
				!bytes.Contains(bts, []byte("MyTeam"))
		}, teatest.WithCheckInterval(time.Millisecond*1), teatest.WithDuration(time.Second*3))

		err := tm.Quit()
		require.NoError(t, err)
	})

	t.Run("shows only breadcrumb when override is empty string", func(t *testing.T) {
		setupTestConfig(t, "version: 1\nworkspace: tea-123\nworkspace_name: MyTeam\n")

		stack := tui.NewStack()
		stack.SetWorkspaceOverride("")
		stack.Push(tui.ModelWithCmd{Model: &testhelper.SimpleModel{Str: "content"}, Breadcrumb: "Workspaces"})
		tm := teatest.NewTestModel(t, stack)

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Workspaces")) &&
				!bytes.Contains(bts, []byte("MyTeam"))
		}, teatest.WithCheckInterval(time.Millisecond*1), teatest.WithDuration(time.Second*3))

		err := tm.Quit()
		require.NoError(t, err)
	})
}

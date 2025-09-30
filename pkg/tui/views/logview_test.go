package views_test

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/render-oss/cli/cmd"
	"github.com/render-oss/cli/pkg/client"
	lclient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/testhelper"
	"github.com/render-oss/cli/pkg/tui/views"
	"github.com/stretchr/testify/require"
)

func TestLogView(t *testing.T) {
	loadFunc := func(_ context.Context, _ views.LogInput) (*tui.LogResult, error) {
		return &tui.LogResult{
			Logs: &client.Logs200Response{
				Logs: []lclient.Log{
					{
						Timestamp: time.Now(),
						Message:   "Hello, world!",
					},
					{
						Timestamp: time.Now(),
						Message:   "Goodbye, world!",
					},
				},
			},
		}, nil
	}

	t.Run("Displays logs", func(t *testing.T) {
		ctx := context.Background()

		input := views.LogInput{
			ResourceIDs: []string{"foo"},
		}

		interactiveLogsCommand := func(ctx context.Context, input views.LogInput, breadcrumb string) tea.Cmd {
			return nil
		}

		logCmd := cmd.NewLogsCmd(nil)

		m := views.NewLogsView(ctx, logCmd, interactiveLogsCommand, input, loadFunc)
		tm := teatest.NewTestModel(t, testhelper.Stackify(m))

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 80})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Hello, world!")) && bytes.Contains(bts, []byte("Goodbye, world!"))
		}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

		err := tm.Quit()
		require.NoError(t, err)
	})

	t.Run("filters logs", func(t *testing.T) {
		ctx := context.Background()

		input := views.LogInput{
			ResourceIDs: []string{"foo"},
		}

		var interactiveInput views.LogInput
		interactiveLogsCommand := func(ctx context.Context, input views.LogInput, breadcrumb string) tea.Cmd {
			interactiveInput = input
			return nil
		}

		logCmd := cmd.NewLogsCmd(nil)

		m := views.NewLogsView(ctx, logCmd, interactiveLogsCommand, input, loadFunc)
		tm := teatest.NewTestModel(t, testhelper.Stackify(m))

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 80})

		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("foo"))
		}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

		// Remove foo
		tm.Send(tea.KeyMsg{Type: tea.KeyBackspace})
		tm.Send(tea.KeyMsg{Type: tea.KeyBackspace})
		tm.Send(tea.KeyMsg{Type: tea.KeyBackspace})

		// Add bar
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bar")})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("bar"))
		}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

		// Search
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

		require.Eventually(t, func() bool {
			return len(interactiveInput.ResourceIDs) == 1 && slices.Contains(interactiveInput.ResourceIDs, "bar")
		}, time.Second*3, time.Millisecond*10)

		err := tm.Quit()
		require.NoError(t, err)
	})
}

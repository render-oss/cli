package tui_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/renderinc/render-cli/pkg/client"
	lclient "github.com/renderinc/render-cli/pkg/client/logs"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/renderinc/render-cli/pkg/tui/testhelper"
	"github.com/stretchr/testify/require"
)

func TestNewLogModel(t *testing.T) {
	filter := tui.NewFilterModel(huh.NewForm(huh.NewGroup(huh.NewInput())), func(form *huh.Form) tea.Cmd {
		return nil
	})

	t.Run("Displays logs", func(t *testing.T) {
		loadFunc := func(_ context.Context, _ any) (*tui.LogResult, error) {
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

		m := tui.NewLogModel(filter, command.LoadCmd(context.Background(), loadFunc, nil))
		tm := teatest.NewTestModel(t, testhelper.Stackify(m))

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Hello, world!")) && bytes.Contains(bts, []byte("Goodbye, world!"))
		}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

		err := tm.Quit()
		require.NoError(t, err)
	})

	t.Run("Tails logs", func(t *testing.T) {
		loadFunc := func(_ context.Context, _ any) (*tui.LogResult, error) {
			ch := make(chan *lclient.Log)
			go func() {
				ch <- &lclient.Log{
					Timestamp: time.Now(),
					Message:   "Hello, world!",
				}
				ch <- &lclient.Log{
					Timestamp: time.Now(),
					Message:   "Goodbye, world!",
				}
				close(ch)
			}()
			return &tui.LogResult{
				Logs:       nil,
				LogChannel: ch,
			}, nil
		}

		m := tui.NewLogModel(filter, command.LoadCmd(context.Background(), loadFunc, nil))
		tm := teatest.NewTestModel(t, testhelper.Stackify(m))

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Hello, world!")) && bytes.Contains(bts, []byte("Goodbye, world!"))
		}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

		err := tm.Quit()
		require.NoError(t, err)
	})

	t.Run("When channel closes, allow refresh", func(t *testing.T) {
		count := 0
		loadFunc := func(_ context.Context, _ any) (*tui.LogResult, error) {
			count++
			ch := make(chan *lclient.Log)
			go func() {
				if count == 1 {
					ch <- &lclient.Log{
						Timestamp: time.Now(),
						Message:   "Hello, world!",
					}
				} else if count == 2 {
					ch <- &lclient.Log{
						Timestamp: time.Now(),
						Message:   "Goodbye, world!",
					}
				}
				close(ch)
			}()
			return &tui.LogResult{
				Logs:       nil,
				LogChannel: ch,
			}, nil
		}

		m := tui.NewLogModel(filter, command.LoadCmd(context.Background(), loadFunc, nil))
		tm := teatest.NewTestModel(t, testhelper.Stackify(m))

		tm.Send(tea.WindowSizeMsg{Width: 100, Height: 24})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Hello, world!")) && bytes.Contains(bts, []byte("Websocket connection closed, no more logs will be displayed. Press 'r' to reload."))
		}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Goodbye, world!"))
		}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

		err := tm.Quit()
		require.NoError(t, err)

		require.Equal(t, 2, count)
	})

	t.Run("Empty state", func(t *testing.T) {
		t.Run("When not tailing", func(t *testing.T) {
			loadFunc := func(_ context.Context, _ any) (*tui.LogResult, error) {
				return &tui.LogResult{
					Logs: &client.Logs200Response{},
				}, nil
			}

			m := tui.NewLogModel(filter, command.LoadCmd(context.Background(), loadFunc, nil))
			tm := teatest.NewTestModel(t, testhelper.Stackify(m))

			tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

			teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
				return bytes.Contains(bts, []byte("No logs to show."))
			}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

			err := tm.Quit()
			require.NoError(t, err)
		})

		t.Run("When tailing", func(t *testing.T) {
			loadFunc := func(_ context.Context, _ any) (*tui.LogResult, error) {
				return &tui.LogResult{
					Logs:       &client.Logs200Response{},
					LogChannel: make(<-chan *lclient.Log),
				}, nil
			}

			m := tui.NewLogModel(filter, command.LoadCmd(context.Background(), loadFunc, nil))
			tm := teatest.NewTestModel(t, testhelper.Stackify(m))

			tm.Send(tea.WindowSizeMsg{Width: 100, Height: 24})

			teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
				return bytes.Contains(bts, []byte("No logs to show. New log entries that match your search parameters will appear here."))
			}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

			err := tm.Quit()
			require.NoError(t, err)
		})
	})
}

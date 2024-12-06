package tui_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/renderinc/cli/pkg/client"
	lclient "github.com/renderinc/cli/pkg/client/logs"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/tui"
	"github.com/renderinc/cli/pkg/tui/testhelper"
	"github.com/stretchr/testify/require"
)

func TestNewLogModel(t *testing.T) {
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

		m := tui.NewLogModel(command.LoadCmd(context.Background(), loadFunc, nil))
		m.SetWidth(80)
		m.SetHeight(24)

		tm := teatest.NewTestModel(t, testhelper.Stackify(m))

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

		m := tui.NewLogModel(command.LoadCmd(context.Background(), loadFunc, nil))
		m.SetWidth(80)
		m.SetHeight(24)

		tm := teatest.NewTestModel(t, testhelper.Stackify(m))

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

		m := tui.NewLogModel(command.LoadCmd(context.Background(), loadFunc, nil))
		m.SetWidth(100)
		m.SetHeight(24)

		tm := teatest.NewTestModel(t, testhelper.Stackify(m))

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

			m := tui.NewLogModel(command.LoadCmd(context.Background(), loadFunc, nil))
			m.SetWidth(80)
			m.SetHeight(24)

			tm := teatest.NewTestModel(t, testhelper.Stackify(m))

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

			m := tui.NewLogModel(command.LoadCmd(context.Background(), loadFunc, nil))
			m.SetWidth(80)
			m.SetHeight(24)

			tm := teatest.NewTestModel(t, testhelper.Stackify(m))

			teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
				return bytes.Contains(bts, []byte("No logs to show. New log entries that match your search parameters will appear here."))
			}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

			err := tm.Quit()
			require.NoError(t, err)
		})
	})
}

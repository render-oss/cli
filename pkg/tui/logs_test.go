package tui_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/render-oss/cli/pkg/client"
	lclient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/testhelper"
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

	t.Run("Pagination", func(t *testing.T) {
		t.Run("Does not auto-fetch on initial load", func(t *testing.T) {
			loadCount := 0
			loadFunc := func(_ context.Context, _ any) (*tui.LogResult, error) {
				loadCount++
				return &tui.LogResult{
					Logs: &client.Logs200Response{
						Logs: []lclient.Log{
							{
								Timestamp: time.Now(),
								Message:   "Log message",
							},
						},
						HasMore:       true,
						NextStartTime: time.Now().Add(-time.Hour),
						NextEndTime:   time.Now(),
					},
				}, nil
			}

			m := tui.NewLogModel(command.LoadCmd(context.Background(), loadFunc, nil))
			m.SetWidth(80)
			m.SetHeight(24)
			m.SetDirection(lclient.Backward)

			// Set up pagination function (normally done by view)
			m.SetLoadMoreFunc(func(startTime, endTime *time.Time) tea.Cmd {
				return func() tea.Msg {
					data, _ := loadFunc(context.Background(), nil)
					return tui.LoadDataMsg[*tui.LogResult]{Data: data}
				}
			})

			tm := teatest.NewTestModel(t, testhelper.Stackify(m))

			// Wait for initial load
			teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
				return bytes.Contains(bts, []byte("Log message"))
			}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

			// Should only have loaded once (no auto-fetch)
			require.Equal(t, 1, loadCount)

			err := tm.Quit()
			require.NoError(t, err)
		})

		t.Run("Loads older logs even when immediately scrolling up after load", func(t *testing.T) {
			loadCount := 0
			loadFunc := func(_ context.Context, _ any) (*tui.LogResult, error) {
				loadCount++
				var logs []lclient.Log
				if loadCount == 1 {
					logs = []lclient.Log{
						{Timestamp: time.Now(), Message: "Initial log 1"},
						{Timestamp: time.Now(), Message: "Initial log 2"},
					}
				} else {
					logs = []lclient.Log{
						{Timestamp: time.Now().Add(-time.Hour), Message: "Older log 1"},
						{Timestamp: time.Now().Add(-time.Hour), Message: "Older log 2"},
					}
				}
				return &tui.LogResult{
					Logs: &client.Logs200Response{
						Logs:          logs,
						HasMore:       loadCount == 1,
						NextStartTime: time.Now().Add(-2 * time.Hour),
						NextEndTime:   time.Now().Add(-time.Hour),
					},
				}, nil
			}

			m := tui.NewLogModel(command.LoadCmd(context.Background(), loadFunc, nil))
			m.SetWidth(80)
			m.SetHeight(24)
			m.SetDirection(lclient.Backward)

			m.SetLoadMoreFunc(func(startTime, endTime *time.Time) tea.Cmd {
				return func() tea.Msg {
					data, _ := loadFunc(context.Background(), nil)
					return tui.LoadDataMsg[*tui.LogResult]{Data: data}
				}
			})

			tm := teatest.NewTestModel(t, testhelper.Stackify(m))

			// Wait for initial load
			teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
				return bytes.Contains(bts, []byte("Initial log 1")) && !bytes.Contains(bts, []byte("Older log 1"))
			}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

			require.Equal(t, 1, loadCount)

			// Scroll up (viewport is already at top and has not changed)
			tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})

			// Wait for pagination to load older logs
			teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
				return bytes.Contains(bts, []byte("Older log 1"))
			}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

			require.Equal(t, 2, loadCount)

			err := tm.Quit()
			require.NoError(t, err)
		})

		t.Run("Does not paginate when hasMore is false", func(t *testing.T) {
			loadCount := 0
			loadFunc := func(_ context.Context, _ any) (*tui.LogResult, error) {
				loadCount++
				return &tui.LogResult{
					Logs: &client.Logs200Response{
						Logs: []lclient.Log{
							{Timestamp: time.Now(), Message: "Only log"},
						},
						HasMore:       false,
						NextStartTime: time.Now().Add(-time.Hour),
						NextEndTime:   time.Now(),
					},
				}, nil
			}

			m := tui.NewLogModel(command.LoadCmd(context.Background(), loadFunc, nil))
			m.SetWidth(80)
			m.SetHeight(24)
			m.SetDirection(lclient.Backward)

			m.SetLoadMoreFunc(func(startTime, endTime *time.Time) tea.Cmd {
				return func() tea.Msg {
					data, _ := loadFunc(context.Background(), nil)
					return tui.LoadDataMsg[*tui.LogResult]{Data: data}
				}
			})

			tm := teatest.NewTestModel(t, testhelper.Stackify(m))

			// Wait for initial load
			teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
				return bytes.Contains(bts, []byte("Only log"))
			}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

			require.Equal(t, 1, loadCount)

			// Try to trigger pagination
			tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
			time.Sleep(100 * time.Millisecond)

			// Should still only have loaded once
			require.Equal(t, 1, loadCount)

			err := tm.Quit()
			require.NoError(t, err)
		})

		t.Run("Does not paginate while tailing logs", func(t *testing.T) {
			loadCount := 0
			loadFunc := func(_ context.Context, _ any) (*tui.LogResult, error) {
				loadCount++
				ch := make(chan *lclient.Log)
				go func() {
					ch <- &lclient.Log{
						Timestamp: time.Now(),
						Message:   "Streaming log",
					}
					time.Sleep(100 * time.Millisecond)
					close(ch)
				}()
				return &tui.LogResult{
					Logs:       &client.Logs200Response{},
					LogChannel: ch,
				}, nil
			}

			m := tui.NewLogModel(command.LoadCmd(context.Background(), loadFunc, nil))
			m.SetWidth(80)
			m.SetHeight(24)
			m.SetDirection(lclient.Backward)

			m.SetLoadMoreFunc(func(startTime, endTime *time.Time) tea.Cmd {
				return func() tea.Msg {
					data, _ := loadFunc(context.Background(), nil)
					return tui.LoadDataMsg[*tui.LogResult]{Data: data}
				}
			})

			tm := teatest.NewTestModel(t, testhelper.Stackify(m))

			// Wait for streaming log
			teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
				return bytes.Contains(bts, []byte("Streaming log"))
			}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

			require.Equal(t, 1, loadCount)

			// Try to trigger pagination while tailing
			tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
			time.Sleep(100 * time.Millisecond)

			// Should still only have loaded once (no pagination during tail)
			require.Equal(t, 1, loadCount)

			err := tm.Quit()
			require.NoError(t, err)
		})

		t.Run("Forward direction loads newer logs at bottom", func(t *testing.T) {
			loadCount := 0
			loadFunc := func(_ context.Context, _ any) (*tui.LogResult, error) {
				loadCount++
				var logs []lclient.Log
				if loadCount == 1 {
					// Initial load with enough logs that the viewport needs to scroll
					logs = make([]lclient.Log, 30)
					for i := range logs {
						logs[i] = lclient.Log{
							Timestamp: time.Now().Add(-2 * time.Hour),
							Message:   "Old log",
						}
					}
				} else {
					// Pagination load (newer logs)
					logs = []lclient.Log{
						{Timestamp: time.Now().Add(-30 * time.Minute), Message: "Newer log 1"},
						{Timestamp: time.Now(), Message: "Newer log 2"},
					}
				}
				return &tui.LogResult{
					Logs: &client.Logs200Response{
						Logs:          logs,
						HasMore:       loadCount == 1,
						NextStartTime: time.Now().Add(-30 * time.Minute),
						NextEndTime:   time.Now(),
					},
				}, nil
			}

			m := tui.NewLogModel(command.LoadCmd(context.Background(), loadFunc, nil))
			m.SetWidth(80)
			m.SetHeight(10) // small height to ensure scrolling is required
			m.SetDirection(lclient.Forward)

			m.SetLoadMoreFunc(func(startTime, endTime *time.Time) tea.Cmd {
				return func() tea.Msg {
					data, _ := loadFunc(context.Background(), nil)
					return tui.LoadDataMsg[*tui.LogResult]{Data: data}
				}
			})

			tm := teatest.NewTestModel(t, testhelper.Stackify(m))

			// Wait for initial load
			teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
				return bytes.Contains(bts, []byte("Old log")) && !bytes.Contains(bts, []byte("Newer log 1"))
			}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

			require.Equal(t, 1, loadCount)

			// Scroll down many times to reach bottom
			for range 10 {
				tm.Send(tea.KeyMsg{Type: tea.KeyPgDown})
				time.Sleep(30 * time.Millisecond)
			}

			// Wait for pagination to load newer logs (with more time since scrolling takes longer)
			teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
				return bytes.Contains(bts, []byte("Newer log 1"))
			}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*5))

			// loadCount should be 2 (initial + pagination)
			require.GreaterOrEqual(t, loadCount, 2, "Expected at least 2 loads (initial + pagination)")

			err := tm.Quit()
			require.NoError(t, err)
		})
	})
}

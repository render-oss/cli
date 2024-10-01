package tui_test

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/renderinc/render-cli/pkg/client"
	lclient "github.com/renderinc/render-cli/pkg/client/logs"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/stretchr/testify/require"
)

func TestNewLogModel(t *testing.T) {
	filter := tui.NewFilterModel(huh.NewForm(huh.NewGroup(huh.NewInput())), func(form *huh.Form) tea.Cmd {
		return nil
	})

	t.Run("Displays logs", func(t *testing.T) {
		loadFunc := func() (*client.Logs200Response, <-chan *lclient.Log, error) {
			return &client.Logs200Response{
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
			}, nil, nil
		}

		m := tui.NewLogModel(filter, loadFunc)
		tm := teatest.NewTestModel(t, m)

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Hello, world!")) && bytes.Contains(bts, []byte("Goodbye, world!"))
		}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

		err := tm.Quit()
		require.NoError(t, err)
	})

	t.Run("Tails logs", func(t *testing.T) {
		loadFunc := func() (*client.Logs200Response, <-chan *lclient.Log, error) {
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
			return nil, ch, nil
		}

		m := tui.NewLogModel(filter, loadFunc)
		tm := teatest.NewTestModel(t, m)

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Hello, world!")) && bytes.Contains(bts, []byte("Goodbye, world!"))
		}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

		err := tm.Quit()
		require.NoError(t, err)
	})
}

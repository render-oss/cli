package tui_test

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/testhelper"
	"github.com/stretchr/testify/require"
)

func TestTabs(t *testing.T) {
	tabs := []*tui.Tab{
		{
			Name:    "Tab 1",
			Content: &testhelper.FakeDimensionModel{Value: "foo"},
		},
		{
			Name:    "Tab 2",
			Content: &testhelper.FakeDimensionModel{Value: "bar"},
		},
	}

	t.Run("displays tabs", func(t *testing.T) {
		tabModel := tui.NewTabModel(tabs)

		tm := teatest.NewTestModel(t, tabModel)

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Tab 1")) && bytes.Contains(bts, []byte("Tab 2"))
		})
	})

	t.Run("switches tabs", func(t *testing.T) {
		tabModel := tui.NewTabModel(tabs)

		tm := teatest.NewTestModel(t, tabModel)

		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("foo")) && !bytes.Contains(bts, []byte("bar"))
		}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

		tm.Send(tea.KeyMsg{Type: tea.KeyShiftRight})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("bar")) && !bytes.Contains(bts, []byte("foo"))
		}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

		tm.Send(tea.KeyMsg{Type: tea.KeyShiftLeft})

		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("foo")) && !bytes.Contains(bts, []byte("bar"))
		}, teatest.WithCheckInterval(time.Millisecond*10), teatest.WithDuration(time.Second*3))

		err := tm.Quit()
		require.NoError(t, err)
	})
}

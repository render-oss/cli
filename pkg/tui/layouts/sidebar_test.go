package layouts_test

import (
	"testing"

	"github.com/renderinc/cli/pkg/tui"
	"github.com/renderinc/cli/pkg/tui/layouts"
	"github.com/renderinc/cli/pkg/tui/testhelper"
	"github.com/stretchr/testify/require"
)

func TestSidebarLayout(t *testing.T) {
	t.Run("sidebar hidden", func(t *testing.T) {
		sidebar := layouts.NewSidebarLayout(
			&testhelper.FakeDimensionModel{Value: "foo"},
			&testhelper.FakeDimensionModel{Value: "bar"},
			&testhelper.FakeDimensionModel{Value: "baz"},
		)

		sidebar.Update(tui.StackSizeMsg{Width: 20, Height: 3})

		view := sidebar.View()
		require.Contains(t, view, "bar")
		require.Contains(t, view, "baz")
		require.NotContains(t, view, "foo")
	})

	t.Run("sidebar visible", func(t *testing.T) {
		sidebar := layouts.NewSidebarLayout(
			&testhelper.FakeDimensionModel{Value: "foo"},
			&testhelper.FakeDimensionModel{Value: "bar"},
			&testhelper.FakeDimensionModel{Value: "baz"},
		)

		sidebar.Update(tui.StackSizeMsg{Width: 20, Height: 3})
		sidebar.SetSidebarVisible(true)

		view := sidebar.View()
		require.Contains(t, view, "bar")
		require.Contains(t, view, "baz")
		require.Contains(t, view, "foo")
	})

	t.Run("children receive width and height", func(t *testing.T) {
		sidebar := &testhelper.FakeDimensionModel{Value: "foo"}
		content := &testhelper.FakeDimensionModel{Value: "bar"}
		footer := &testhelper.FakeDimensionModel{Value: "baz"}

		layout := layouts.NewSidebarLayout(sidebar, content, footer)
		layout.SetFooterHeight(1)
		layout.SetSidebarWidth(5)

		layout.Update(tui.StackSizeMsg{Width: 20, Height: 3})

		require.Equal(t, 20, content.Width)
		require.Equal(t, 2, content.Height)

		require.Equal(t, 20, footer.Width)
		require.Equal(t, 1, footer.Height)

		layout.SetSidebarVisible(true)

		require.Equal(t, 5, sidebar.Width)
		require.Equal(t, 2, sidebar.Height)

		require.Equal(t, 15, content.Width)
		require.Equal(t, 2, content.Height)
	})
}

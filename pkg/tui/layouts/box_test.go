package layouts_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/render-oss/cli/pkg/tui/layouts"
	"github.com/render-oss/cli/pkg/tui/testhelper"
	"github.com/stretchr/testify/require"
)

func TestBoxLayout(t *testing.T) {
	t.Run("properly calculates interior width and height", func(t *testing.T) {
		child := testhelper.FakeDimensionModel{Value: "foo"}

		style := lipgloss.NewStyle().Padding(1, 2, 3, 4).Margin(1, 2, 3, 4)

		box := layouts.NewBoxLayout(style, &child)

		box.SetWidth(20)
		box.SetHeight(20)

		// The box should have right padding and margin of 2 and left padding and margin of 4 for a total of 12
		require.Equal(t, 20-12, child.Width)

		// The box should have top padding and margin of 1 and bottom padding and margin of 3 for a total of 8
		require.Equal(t, 20-8, child.Height)
	})
}

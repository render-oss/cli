package command_test

import (
	"testing"

	"github.com/render-oss/cli/pkg/command"
	"github.com/stretchr/testify/require"
)

func TestCobraEnum(t *testing.T) {
	t.Run("single select", func(t *testing.T) {
		t.Run("properties", func(t *testing.T) {
			enum := command.NewEnumInput([]string{"a", "b", "c"}, false)

			require.False(t, enum.IsMultiSelect())
			require.Equal(t, "enum", enum.Type())
			require.Equal(t, []string{"a", "b", "c"}, enum.Values())
		})

		t.Run("can set to valid value", func(t *testing.T) {
			enum := command.NewEnumInput([]string{"a", "b", "c"}, false)

			require.NoError(t, enum.Set("b"))
			require.Equal(t, []string{"b"}, enum.SelectedValues())
		})

		t.Run("is case insensitive", func(t *testing.T) {
			enum := command.NewEnumInput([]string{"a", "b", "c"}, false)

			require.NoError(t, enum.Set("B"))
			require.Equal(t, []string{"b"}, enum.SelectedValues())
		})

		t.Run("errors when set to invalid value", func(t *testing.T) {
			enum := command.NewEnumInput([]string{"a", "b", "c"}, false)

			require.Error(t, enum.Set("d"))
			require.Empty(t, enum.SelectedValues())
		})
	})

	t.Run("multi select", func(t *testing.T) {
		t.Run("properties", func(t *testing.T) {
			enum := command.NewEnumInput([]string{"a", "b", "c"}, true)

			require.True(t, enum.IsMultiSelect())
			require.Equal(t, "enum", enum.Type())
			require.Equal(t, []string{"a", "b", "c"}, enum.Values())
		})

		t.Run("can set to valid value", func(t *testing.T) {
			enum := command.NewEnumInput([]string{"a", "b", "c"}, true)

			require.NoError(t, enum.Set("b,c"))
			require.Equal(t, []string{"b", "c"}, enum.SelectedValues())
		})

		t.Run("is case insensitive", func(t *testing.T) {
			enum := command.NewEnumInput([]string{"a", "b", "c"}, true)

			require.NoError(t, enum.Set("B,C"))
			require.Equal(t, []string{"b", "c"}, enum.SelectedValues())
		})

		t.Run("errors when set to invalid value", func(t *testing.T) {
			enum := command.NewEnumInput([]string{"a", "b", "c"}, true)

			require.Error(t, enum.Set("d"))
			require.Empty(t, enum.SelectedValues())
		})
	})
}

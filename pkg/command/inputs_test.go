package command_test

import (
	"testing"

	"github.com/renderinc/render-cli/pkg/command"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestParseCommand(t *testing.T) {
	t.Run("parse basic type", func(t *testing.T) {
		type testStruct struct {
			Foo string `cli:"foo"`
		}
		var v testStruct
		cmd := &cobra.Command{}
		cmd.Flags().String("foo", "", "")
		require.NoError(t, cmd.ParseFlags([]string{"--foo", "bar"}))

		err := command.ParseCommand(cmd, []string{}, &v)
		require.NoError(t, err)

		require.Equal(t, "bar", v.Foo)
	})

	t.Run("parse pointer", func(t *testing.T) {
		type testStruct struct {
			Foo *string `cli:"foo"`
		}
		var v testStruct
		cmd := &cobra.Command{}
		cmd.Flags().String("foo", "", "")
		require.NoError(t, cmd.ParseFlags([]string{"--foo", "bar"}))

		err := command.ParseCommand(cmd, []string{}, &v)
		require.NoError(t, err)

		require.Equal(t, "bar", *v.Foo)
	})

	t.Run("parse slice", func(t *testing.T) {
		type testStruct struct {
			Foo []string `cli:"foo"`
		}
		var v testStruct
		cmd := &cobra.Command{}
		cmd.Flags().StringSlice("foo", []string{}, "")
		require.NoError(t, cmd.ParseFlags([]string{"--foo", "bar,baz"}))

		err := command.ParseCommand(cmd, []string{}, &v)
		require.NoError(t, err)

		require.Equal(t, []string{"bar", "baz"}, v.Foo)
	})

	t.Run("arg parsing", func(t *testing.T) {
		t.Run("simple arg", func(t *testing.T) {
			type testStruct struct {
				Foo string `cli:"arg:0"`
			}
			var v testStruct
			cmd := &cobra.Command{}

			err := command.ParseCommand(cmd, []string{"bar"}, &v)
			require.NoError(t, err)

			require.Equal(t, "bar", v.Foo)
		})

		t.Run("pointer arg", func(t *testing.T) {
			type testStruct struct {
				Foo *string `cli:"arg:0"`
			}
			var v testStruct
			cmd := &cobra.Command{}

			err := command.ParseCommand(cmd, []string{"bar"}, &v)
			require.NoError(t, err)

			require.Equal(t, "bar", *v.Foo)
		})
	})
}

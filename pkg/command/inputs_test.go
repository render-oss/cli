package command_test

import (
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/pointers"
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

	t.Run("parse single select enum", func(t *testing.T) {
		enumInput := command.NewEnumInput([]string{"bar", "baz"}, false)

		type testStruct struct {
			Foo string `cli:"foo"`
		}
		var v testStruct
		cmd := &cobra.Command{}
		cmd.Flags().Var(enumInput, "foo", "")
		require.NoError(t, cmd.ParseFlags([]string{"--foo", "baz"}))

		err := command.ParseCommand(cmd, []string{}, &v)
		require.NoError(t, err)

		require.Equal(t, "baz", v.Foo)
	})

	t.Run("parse multi select enum", func(t *testing.T) {
		enumInput := command.NewEnumInput([]string{"a", "b", "c"}, true)

		type testStruct struct {
			Foo []string `cli:"foo"`
		}
		var v testStruct
		cmd := &cobra.Command{}
		cmd.Flags().Var(enumInput, "foo", "")
		require.NoError(t, cmd.ParseFlags([]string{"--foo", "a,c"}))

		err := command.ParseCommand(cmd, []string{}, &v)
		require.NoError(t, err)

		require.Equal(t, []string{"a", "c"}, v.Foo)
	})

	t.Run("parse time", func(t *testing.T) {
		timeInput := command.NewTimeInput()

		type testStruct struct {
			Foo *command.TimeOrRelative `cli:"foo"`
		}
		var v testStruct
		cmd := &cobra.Command{}
		cmd.Flags().Var(timeInput, "foo", "")
		require.NoError(t, cmd.ParseFlags([]string{"--foo", "5m"}))

		err := command.ParseCommand(cmd, []string{}, &v)
		require.NoError(t, err)

		require.Equal(t, "5m", *v.Foo.Relative)
		require.WithinDuration(t, *v.Foo.T, time.Now().Add(-5*time.Minute), time.Second)
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

func TestInputToString(t *testing.T) {
	t.Run("args", func(t *testing.T) {
		type testStruct struct {
			Foo string  `cli:"arg:0"`
			Bar *string `cli:"arg:1"`
		}

		v := testStruct{Foo: "abc", Bar: pointers.From("def")}
		str, err := command.InputToString(&v)
		require.NoError(t, err)
		require.Equal(t, "abc def", str)
	})

	t.Run("flags", func(t *testing.T) {
		type testStruct struct {
			Foo string   `cli:"foo"`
			Bar *int     `cli:"bar"`
			Baz []string `cli:"baz"`
		}

		v := testStruct{Foo: "abc", Bar: pointers.From(123), Baz: []string{"def", "ghi"}}
		str, err := command.InputToString(&v)
		require.NoError(t, err)
		require.Equal(t, "--foo=abc --bar=123 --baz=def,ghi", str)
	})

	t.Run("args and flags", func(t *testing.T) {
		type testStruct struct {
			Foo  string  `cli:"foo"`
			Bar  *int    `cli:"bar"`
			Arg0 string  `cli:"arg:0"`
			Arg1 *string `cli:"arg:1"`
		}

		v := testStruct{
			Foo:  "abc",
			Bar:  pointers.From(123),
			Arg0: "def",
			Arg1: pointers.From("ghi"),
		}
		str, err := command.InputToString(&v)
		require.NoError(t, err)
		require.Equal(t, "def ghi --foo=abc --bar=123", str)
	})

	t.Run("missing args and flags not represented", func(t *testing.T) {
		type testStruct struct {
			Foo  *string  `cli:"foo"`
			Bar  []string `cli:"bar"`
			Arg0 *string  `cli:"arg:0"`
		}

		v := testStruct{
			Foo:  nil,
			Bar:  []string{},
			Arg0: nil,
		}
		str, err := command.InputToString(&v)
		require.NoError(t, err)
		require.Equal(t, "", str)
	})

	t.Run("zero args and flags not represented", func(t *testing.T) {
		type testStruct struct {
			Foo string `cli:"foo"`
			Bar int    `cli:"bar"`
		}

		v := testStruct{
			Foo: "",
			Bar: 0,
		}
		str, err := command.InputToString(&v)
		require.NoError(t, err)
		require.Equal(t, "", str)
	})
}

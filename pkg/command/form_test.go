package command_test

import (
	"testing"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/renderinc/cli/pkg/command"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestFormValuesFromStruct(t *testing.T) {
	t.Run("converts basic type", func(t *testing.T) {
		type testStruct struct {
			OwnerID string `cli:"owner"`
		}
		v := testStruct{OwnerID: "owner-id"}
		formValues := command.FormValuesFromStruct(&v)
		require.Equal(t, "owner-id", formValues["owner"].String())
	})

	t.Run("converts pointer type", func(t *testing.T) {
		type testStruct struct {
			OwnerID *string `cli:"owner"`
		}
		ownerID := "owner-id"
		v := testStruct{OwnerID: &ownerID}
		formValues := command.FormValuesFromStruct(&v)
		require.Equal(t, "owner-id", formValues["owner"].String())
	})

	t.Run("converts slice type", func(t *testing.T) {
		type testStruct struct {
			OwnerIDs []string `cli:"owners"`
		}
		v := testStruct{OwnerIDs: []string{"owner-id-1", "owner-id-2"}}
		formValues := command.FormValuesFromStruct(&v)
		require.Equal(t, "owner-id-1,owner-id-2", formValues["owners"].String())
	})
}

func TestStructFromFormValues(t *testing.T) {
	str := "owner-id"

	t.Run("converts basic type", func(t *testing.T) {
		type testStruct struct {
			OwnerID string `cli:"owner"`
		}
		formValues := command.FormValues{"owner": command.NewStringFormValue(str)}
		v := testStruct{}
		require.NoError(t, command.StructFromFormValues(formValues, &v))
		require.Equal(t, "owner-id", v.OwnerID)
	})

	t.Run("converts pointer type", func(t *testing.T) {
		type testStruct struct {
			OwnerID *string `cli:"owner"`
		}
		formValues := command.FormValues{"owner": command.NewStringFormValue(str)}
		v := testStruct{}
		require.NoError(t, command.StructFromFormValues(formValues, &v))
		require.Equal(t, "owner-id", *v.OwnerID)
	})

	t.Run("converts slice type", func(t *testing.T) {
		type testStruct struct {
			OwnerIDs []string `cli:"owners"`
		}
		strSlice := "owner-id-1,owner-id-2"
		formValues := command.FormValues{"owners": command.NewStringSliceFormValue(strSlice)}
		v := testStruct{}
		require.NoError(t, command.StructFromFormValues(formValues, &v))
		require.Equal(t, []string{"owner-id-1", "owner-id-2"}, v.OwnerIDs)
	})

	t.Run("converts time type", func(t *testing.T) {
		type testStruct struct {
			Time *command.TimeOrRelative `cli:"time"`
		}
		str := "1m"
		formValues := command.FormValues{"time": command.NewStringFormValue(str)}
		v := testStruct{}
		require.NoError(t, command.StructFromFormValues(formValues, &v))
		require.Equal(t, "1m", v.Time.String())
		require.WithinDuration(t, *v.Time.T, time.Now().Add(-time.Minute), time.Second)
	})

	t.Run("converts enum type", func(t *testing.T) {
		type testStruct struct {
			Foo string `cli:"foo"`
		}
		formValues := command.FormValues{"foo": command.NewStringFormValue("value")}
		v := testStruct{}
		require.NoError(t, command.StructFromFormValues(formValues, &v))
		require.Equal(t, "value", v.Foo)
	})

	t.Run("converts enum multi type", func(t *testing.T) {
		type testStruct struct {
			Foo []string `cli:"foo"`
		}
		formValues := command.FormValues{"foo": command.NewStringFormValue("value,other")}
		v := testStruct{}
		require.NoError(t, command.StructFromFormValues(formValues, &v))
		require.Equal(t, []string{"value", "other"}, v.Foo)
	})
}

func TestHuhForm(t *testing.T) {
	t.Run("creates form", func(t *testing.T) {
		type testStruct struct {
			Foo string `cli:"foo"`
			Bar int    `cli:"bar"`
		}
		v := testStruct{}
		cmd := cobra.Command{}
		cmd.Flags().String("foo", "", "")
		cmd.Flags().Int("bar", 0, "")

		fields, _ := command.HuhFormFields(&cmd, &v)
		form := huh.NewForm(huh.NewGroup(fields...))
		form.Init()()

		require.Contains(t, form.View(), "foo")
		require.Contains(t, form.View(), "bar")
	})

	t.Run("creates form with enums", func(t *testing.T) {
		type testStruct struct {
			Foo string `cli:"foo"`
			Bar int    `cli:"bar"`
		}
		v := testStruct{}
		cmd := cobra.Command{}

		// foo is multi select
		fooInput := command.NewEnumInput([]string{"multi choice 1", "multi choice 2", "multi choice 3"}, true)
		cmd.Flags().Var(fooInput, "foo", "")

		// bar is single select
		barInput := command.NewEnumInput([]string{"single choice 1", "single choice 2", "single choice 3"}, false)
		cmd.Flags().Var(barInput, "bar", "")

		fields, _ := command.HuhFormFields(&cmd, &v)
		form := huh.NewForm(huh.NewGroup(fields...))
		form.Init()

		require.Contains(t, form.View(), "multi choice 3")
		require.Contains(t, form.View(), "single choice 2")
	})

	t.Run("creates form with time", func(t *testing.T) {
		type testStruct struct {
			Foo *command.TimeOrRelative `cli:"foo"`
		}
		v := testStruct{}
		cmd := cobra.Command{}

		// foo is multi select
		fooInput := command.NewTimeInput()
		cmd.Flags().Var(fooInput, "foo", "")

		fields, _ := command.HuhFormFields(&cmd, &v)
		form := huh.NewForm(huh.NewGroup(fields...))
		form.Init()

		require.Contains(t, form.View(), "foo")
		// Find placeholder text
		require.Contains(t, form.View(), "Relative time or")
	})
}

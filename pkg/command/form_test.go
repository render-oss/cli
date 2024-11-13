package command_test

import (
	"testing"

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
		require.Equal(t, "owner-id", *formValues["owner"])
	})

	t.Run("converts pointer type", func(t *testing.T) {
		type testStruct struct {
			OwnerID *string `cli:"owner"`
		}
		ownerID := "owner-id"
		v := testStruct{OwnerID: &ownerID}
		formValues := command.FormValuesFromStruct(&v)
		require.Equal(t, "owner-id", *formValues["owner"])
	})

	t.Run("converts slice type", func(t *testing.T) {
		type testStruct struct {
			OwnerIDs []string `cli:"owners"`
		}
		v := testStruct{OwnerIDs: []string{"owner-id-1", "owner-id-2"}}
		formValues := command.FormValuesFromStruct(&v)
		require.Equal(t, "owner-id-1,owner-id-2", *formValues["owners"])
	})
}

func TestStructFromFormValues(t *testing.T) {
	str := "owner-id"

	t.Run("converts basic type", func(t *testing.T) {
		type testStruct struct {
			OwnerID string `cli:"owner"`
		}
		formValues := command.FormValues{"owner": &str}
		v := testStruct{}
		require.NoError(t, command.StructFromFormValues(formValues, &v))
		require.Equal(t, "owner-id", v.OwnerID)
	})

	t.Run("converts pointer type", func(t *testing.T) {
		type testStruct struct {
			OwnerID *string `cli:"owner"`
		}
		formValues := command.FormValues{"owner": &str}
		v := testStruct{}
		require.NoError(t, command.StructFromFormValues(formValues, &v))
		require.Equal(t, "owner-id", *v.OwnerID)
	})

	t.Run("converts slice type", func(t *testing.T) {
		type testStruct struct {
			OwnerIDs []string `cli:"owners"`
		}
		strSlice := "owner-id-1,owner-id-2"
		formValues := command.FormValues{"owners": &strSlice}
		v := testStruct{}
		require.NoError(t, command.StructFromFormValues(formValues, &v))
		require.Equal(t, []string{"owner-id-1", "owner-id-2"}, v.OwnerIDs)
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

		form, _ := command.HuhForm(&cmd, &v)
		form.Init()()

		require.Contains(t, form.View(), "foo")
		require.Contains(t, form.View(), "bar")
	})
}

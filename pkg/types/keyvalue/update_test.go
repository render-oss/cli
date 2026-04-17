package keyvalue_test

import (
	"testing"

	"github.com/render-oss/cli/v2/pkg/pointers"
	kvtypes "github.com/render-oss/cli/v2/pkg/types/keyvalue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAndValidateUpdateInput(t *testing.T) {
	t.Run("Update with Name field passes validation", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			Name: pointers.From("new-name"),
		}
		got, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.NoError(t, err)
		assert.Equal(t, pointers.From("new-name"), got.Name)
		assert.Nil(t, got.Plan)
		assert.Nil(t, got.MaxmemoryPolicy)
	})

	t.Run("Update with Plan field passes validation", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			Plan: pointers.From("pro"),
		}
		got, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.NoError(t, err)
		assert.Nil(t, got.Name)
		assert.Equal(t, pointers.From("pro"), got.Plan)
		assert.Nil(t, got.MaxmemoryPolicy)
	})

	t.Run("Update with MaxmemoryPolicy field passes validation", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			MaxmemoryPolicy: pointers.From(kvtypes.MaxmemoryPolicyNoeviction),
		}
		got, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.NoError(t, err)
		assert.Nil(t, got.Name)
		assert.Nil(t, got.Plan)
		assert.Equal(t, pointers.From(kvtypes.MaxmemoryPolicyNoeviction), got.MaxmemoryPolicy)
	})

	t.Run("Update with multiple fields passes validation", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			Name: pointers.From("updated-name"),
			Plan: pointers.From("standard"),
		}
		got, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.NoError(t, err)
		assert.Equal(t, pointers.From("updated-name"), got.Name)
		assert.Equal(t, pointers.From("standard"), got.Plan)
		assert.Nil(t, got.MaxmemoryPolicy)
	})

	t.Run("Update with all nil fields returns validation error", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			Name:            nil,
			Plan:            nil,
			MaxmemoryPolicy: nil,
		}
		_, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one field must be provided for update")
	})

	t.Run("Update with no fields set returns validation error", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{}
		_, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one field must be provided for update")
	})

	t.Run("Whitespace-only Name is normalized to nil", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			Name: pointers.From("   "),
		}
		_, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one field must be provided for update")
	})

	t.Run("Whitespace-only Plan is normalized to nil", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			Plan: pointers.From("   "),
		}
		_, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one field must be provided for update")
	})

	t.Run("Whitespace-only MaxmemoryPolicy is normalized to nil", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			MaxmemoryPolicy: pointers.From(kvtypes.MaxmemoryPolicy("   ")),
		}
		_, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one field must be provided for update")
	})

	t.Run("Whitespace-only values normalized to nil when all fields whitespace-only", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			Name:            pointers.From("   "),
			Plan:            pointers.From("   "),
			MaxmemoryPolicy: pointers.From(kvtypes.MaxmemoryPolicy("   ")),
		}
		_, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one field must be provided for update")
	})

	t.Run("Name with leading/trailing whitespace is trimmed and preserved", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			Name: pointers.From(" updated-name "),
		}
		got, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.NoError(t, err)
		assert.Equal(t, pointers.From("updated-name"), got.Name)
	})

	t.Run("Plan with leading/trailing whitespace is trimmed and preserved", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			Plan: pointers.From(" pro "),
		}
		got, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.NoError(t, err)
		assert.Equal(t, pointers.From("pro"), got.Plan)
	})

	t.Run("MaxmemoryPolicy with leading/trailing whitespace is trimmed and preserved", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			MaxmemoryPolicy: pointers.From(kvtypes.MaxmemoryPolicy(" allkeys_lru ")),
		}
		got, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.NoError(t, err)
		assert.Equal(t, pointers.From(kvtypes.MaxmemoryPolicyAllkeysLru), got.MaxmemoryPolicy)
	})

	t.Run("Mixed whitespace and non-whitespace fields: one valid, others whitespace", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			Name:            pointers.From("valid-name"),
			Plan:            pointers.From("   "),
			MaxmemoryPolicy: pointers.From(kvtypes.MaxmemoryPolicy("   ")),
		}
		got, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.NoError(t, err)
		assert.Equal(t, pointers.From("valid-name"), got.Name)
		assert.Nil(t, got.Plan)
		assert.Nil(t, got.MaxmemoryPolicy)
	})

	t.Run("All three fields set with valid values passes validation", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			Name:            pointers.From("new-name"),
			Plan:            pointers.From("standard"),
			MaxmemoryPolicy: pointers.From(kvtypes.MaxmemoryPolicyVolatileLru),
		}
		got, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.NoError(t, err)
		assert.Equal(t, pointers.From("new-name"), got.Name)
		assert.Equal(t, pointers.From("standard"), got.Plan)
		assert.Equal(t, pointers.From(kvtypes.MaxmemoryPolicyVolatileLru), got.MaxmemoryPolicy)
	})

	t.Run("IPAllowList alone satisfies at-least-one-field requirement", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			IPAllowList: []string{"cidr=10.0.0.0/8"},
		}
		got, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.NoError(t, err)
		assert.Equal(t, []string{"cidr=10.0.0.0/8"}, got.IPAllowList)
	})

	t.Run("Valid ip-allow-list entry passes validation", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			IPAllowList: []string{"cidr=192.168.1.0/24,description=office"},
		}
		got, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.NoError(t, err)
		assert.Equal(t, []string{"cidr=192.168.1.0/24,description=office"}, got.IPAllowList)
	})

	t.Run("Invalid ip-allow-list entry returns validation error", func(t *testing.T) {
		input := kvtypes.KeyValueUpdateInput{
			IPAllowList: []string{"not-a-cidr"},
		}
		_, err := kvtypes.NormalizeAndValidateUpdateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--ip-allow-list")
	})
}

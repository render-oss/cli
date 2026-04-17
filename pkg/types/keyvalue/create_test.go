package keyvalue_test

import (
	"testing"

	"github.com/render-oss/cli/v2/pkg/pointers"
	kvtypes "github.com/render-oss/cli/v2/pkg/types/keyvalue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAndValidateCreateInput(t *testing.T) {
	t.Run("Valid input with name and plan passes validation", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name: "my-kv",
			Plan: "starter",
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Equal(t, "my-kv", got.Name)
		assert.Equal(t, "starter", got.Plan)
		assert.Nil(t, got.Region)
		assert.Nil(t, got.EnvironmentIDOrName)
		assert.Nil(t, got.MaxmemoryPolicy)
	})

	t.Run("Valid input with name, plan, and optional fields passes validation", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:                "my-kv",
			Plan:                "pro",
			Region:              pointers.From("oregon"),
			EnvironmentIDOrName: pointers.From("env-123"),
			MaxmemoryPolicy:     pointers.From(kvtypes.MaxmemoryPolicyAllkeysLru),
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Equal(t, "my-kv", got.Name)
		assert.Equal(t, "pro", got.Plan)
		assert.Equal(t, pointers.From("oregon"), got.Region)
		assert.Equal(t, pointers.From("env-123"), got.EnvironmentIDOrName)
		assert.Equal(t, pointers.From(kvtypes.MaxmemoryPolicyAllkeysLru), got.MaxmemoryPolicy)
	})

	t.Run("Input with empty name returns validation error", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name: "",
			Plan: "starter",
		}
		_, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("Input with empty plan returns validation error", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name: "my-kv",
			Plan: "",
		}
		_, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "plan is required")
	})

	t.Run("Input with whitespace-only name is normalized and rejected", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name: "   ",
			Plan: "starter",
		}
		_, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("Name with leading/trailing whitespace is trimmed", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name: " my-kv ",
			Plan: "starter",
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Equal(t, "my-kv", got.Name)
	})

	t.Run("Plan with leading/trailing whitespace is trimmed", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name: "my-kv",
			Plan: " pro ",
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Equal(t, "pro", got.Plan)
	})

	t.Run("Whitespace-only optional region field is normalized to nil", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:   "my-kv",
			Plan:   "starter",
			Region: pointers.From("   "),
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Nil(t, got.Region)
	})

	t.Run("Whitespace-only optional environment-id field is normalized to nil", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:                "my-kv",
			Plan:                "starter",
			EnvironmentIDOrName: pointers.From("   "),
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Nil(t, got.EnvironmentIDOrName)
	})

	t.Run("Whitespace-only optional maxmemory-policy field is normalized to nil", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:            "my-kv",
			Plan:            "starter",
			MaxmemoryPolicy: pointers.From(kvtypes.MaxmemoryPolicy("   ")),
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Nil(t, got.MaxmemoryPolicy)
	})

	t.Run("Optional fields with valid values are preserved", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:                "my-kv",
			Plan:                "starter",
			Region:              pointers.From(" oregon "),
			EnvironmentIDOrName: pointers.From(" env-456 "),
			MaxmemoryPolicy:     pointers.From(kvtypes.MaxmemoryPolicy(" noeviction ")),
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		// OptionalNonZeroString should trim whitespace but preserve non-empty values
		assert.Equal(t, pointers.From("oregon"), got.Region)
		assert.Equal(t, pointers.From("env-456"), got.EnvironmentIDOrName)
		assert.Equal(t, pointers.From(kvtypes.MaxmemoryPolicyNoeviction), got.MaxmemoryPolicy)
	})

	t.Run("Input with both name and plan empty returns error for name first", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name: "",
			Plan: "",
		}
		_, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("Valid ip-allow-list entry passes validation", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:        "my-kv",
			Plan:        "starter",
			IPAllowList: []string{"cidr=192.168.1.0/24,description=office"},
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Equal(t, []string{"cidr=192.168.1.0/24,description=office"}, got.IPAllowList)
	})

	t.Run("Invalid ip-allow-list entry returns validation error", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:        "my-kv",
			Plan:        "starter",
			IPAllowList: []string{"not-a-cidr"},
		}
		_, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--ip-allow-list")
	})

	t.Run("Empty ip-allow-list passes validation", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name: "my-kv",
			Plan: "starter",
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Empty(t, got.IPAllowList)
	})
}

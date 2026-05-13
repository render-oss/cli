package keyvalue_test

import (
	"testing"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/pointers"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAndValidateCreateInput(t *testing.T) {
	t.Run("Valid input with name and plan passes validation", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name: "my-kv",
			Plan: kvtypes.PlanStarter,
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Equal(t, "my-kv", got.Name)
		assert.Equal(t, kvtypes.PlanStarter, got.Plan)
		assert.Nil(t, got.Region)
		assert.Nil(t, got.EnvironmentIDOrName)
		assert.Nil(t, got.MaxmemoryPolicy)
	})

	t.Run("Valid input with name, plan, and optional fields passes validation", func(t *testing.T) {
		environmentID := testids.EnvironmentID("optional")
		input := kvtypes.KeyValueCreateInput{
			Name:                "my-kv",
			Plan:                kvtypes.PlanPro,
			Region:              pointers.From("oregon"),
			EnvironmentIDOrName: pointers.From(environmentID),
			MaxmemoryPolicy:     pointers.From(kvtypes.MaxmemoryPolicyAllkeysLru),
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Equal(t, "my-kv", got.Name)
		assert.Equal(t, kvtypes.PlanPro, got.Plan)
		assert.Equal(t, pointers.From("oregon"), got.Region)
		assert.Equal(t, pointers.From(environmentID), got.EnvironmentIDOrName)
		assert.Equal(t, pointers.From(kvtypes.MaxmemoryPolicyAllkeysLru), got.MaxmemoryPolicy)
	})

	t.Run("Input with empty name returns validation error", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name: "",
			Plan: kvtypes.PlanStarter,
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
			Plan: kvtypes.PlanStarter,
		}
		_, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("Name with leading/trailing whitespace is trimmed", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name: " my-kv ",
			Plan: kvtypes.PlanStarter,
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Equal(t, "my-kv", got.Name)
	})

	t.Run("Plan with leading/trailing whitespace is trimmed", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name: "my-kv",
			Plan: kvtypes.Plan(" pro "),
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Equal(t, kvtypes.PlanPro, got.Plan)
	})

	t.Run("Whitespace-only optional region field is normalized to nil", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:   "my-kv",
			Plan:   kvtypes.PlanStarter,
			Region: pointers.From("   "),
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Nil(t, got.Region)
	})

	t.Run("Whitespace-only optional environment-id field is normalized to nil", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:                "my-kv",
			Plan:                kvtypes.PlanStarter,
			EnvironmentIDOrName: pointers.From("   "),
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Nil(t, got.EnvironmentIDOrName)
	})

	t.Run("Whitespace-only optional maxmemory-policy field is normalized to nil", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:            "my-kv",
			Plan:            kvtypes.PlanStarter,
			MaxmemoryPolicy: pointers.From(kvtypes.MaxmemoryPolicy("   ")),
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Nil(t, got.MaxmemoryPolicy)
	})

	t.Run("Optional fields with valid values are preserved", func(t *testing.T) {
		environmentID := testids.EnvironmentID("preserved")
		input := kvtypes.KeyValueCreateInput{
			Name:                "my-kv",
			Plan:                kvtypes.PlanStarter,
			Region:              pointers.From(" oregon "),
			EnvironmentIDOrName: pointers.From(" " + environmentID + " "),
			MaxmemoryPolicy:     pointers.From(kvtypes.MaxmemoryPolicy(" noeviction ")),
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		// OptionalNonZeroString should trim whitespace but preserve non-empty values
		assert.Equal(t, pointers.From("oregon"), got.Region)
		assert.Equal(t, pointers.From(environmentID), got.EnvironmentIDOrName)
		assert.Equal(t, pointers.From(kvtypes.MaxmemoryPolicyNoeviction), got.MaxmemoryPolicy)
	})

	t.Run("Input with both name and plan empty returns error for name first", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name: "",
			Plan: kvtypes.Plan(""),
		}
		_, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("Valid ip-allow-list entry passes validation", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:        "my-kv",
			Plan:        kvtypes.PlanStarter,
			IPAllowList: []string{"cidr=192.168.1.0/24,description=office"},
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Equal(t, []string{"cidr=192.168.1.0/24,description=office"}, got.IPAllowList)
	})

	t.Run("Invalid ip-allow-list entry returns validation error", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:        "my-kv",
			Plan:        kvtypes.PlanStarter,
			IPAllowList: []string{"not-a-cidr"},
		}
		_, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--ip-allow-list")
	})

	t.Run("Empty ip-allow-list passes validation", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name: "my-kv",
			Plan: kvtypes.PlanStarter,
		}
		got, err := kvtypes.NormalizeAndValidateCreateInput(input)
		require.NoError(t, err)
		assert.Empty(t, got.IPAllowList)
	})
}

func TestNormalizeCreateInput_WorkspaceIDOrName(t *testing.T) {
	t.Run("trims whitespace from WorkspaceIDOrName", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:              "my-kv",
			Plan:              kvtypes.PlanFree,
			WorkspaceIDOrName: "  my-workspace  ",
		}
		got := kvtypes.NormalizeCreateInput(input)
		assert.Equal(t, "my-workspace", got.WorkspaceIDOrName)
	})

	t.Run("empty WorkspaceIDOrName is left as-is (empty means: use config default)", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:              "my-kv",
			Plan:              kvtypes.PlanFree,
			WorkspaceIDOrName: "",
		}
		got := kvtypes.NormalizeCreateInput(input)
		assert.Equal(t, "", got.WorkspaceIDOrName)
	})
}

func TestNormalizeCreateInput_MemoryPolicyShortcuts(t *testing.T) {
	t.Run("cache shortcut normalizes to allkeys_lru", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:            "my-kv",
			Plan:            kvtypes.PlanFree,
			MaxmemoryPolicy: pointers.From(kvtypes.MaxmemoryPolicyCache),
		}
		got := kvtypes.NormalizeCreateInput(input)
		require.NotNil(t, got.MaxmemoryPolicy)
		assert.Equal(t, kvtypes.MaxmemoryPolicyAllkeysLru, *got.MaxmemoryPolicy)
	})

	t.Run("queue shortcut normalizes to noeviction", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:            "my-kv",
			Plan:            kvtypes.PlanFree,
			MaxmemoryPolicy: pointers.From(kvtypes.MaxmemoryPolicyQueue),
		}
		got := kvtypes.NormalizeCreateInput(input)
		require.NotNil(t, got.MaxmemoryPolicy)
		assert.Equal(t, kvtypes.MaxmemoryPolicyNoeviction, *got.MaxmemoryPolicy)
	})

	t.Run("technical value passes through unchanged", func(t *testing.T) {
		input := kvtypes.KeyValueCreateInput{
			Name:            "my-kv",
			Plan:            kvtypes.PlanFree,
			MaxmemoryPolicy: pointers.From(kvtypes.MaxmemoryPolicyAllkeysLru),
		}
		got := kvtypes.NormalizeCreateInput(input)
		require.NotNil(t, got.MaxmemoryPolicy)
		assert.Equal(t, kvtypes.MaxmemoryPolicyAllkeysLru, *got.MaxmemoryPolicy)
	})
}

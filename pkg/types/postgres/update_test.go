package postgres_test

import (
	"testing"

	"github.com/render-oss/cli/pkg/pointers"
	postgrestypes "github.com/render-oss/cli/pkg/types/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdatePostgresInputValidate(t *testing.T) {
	t.Run("requires ID or name argument", func(t *testing.T) {
		input := postgrestypes.UpdatePostgresInput{Name: pointers.From("new-name")}
		err := input.Validate(false)
		assert.ErrorContains(t, err, "postgres ID or name argument is required")
	})

	t.Run("requires at least one field to update", func(t *testing.T) {
		input := postgrestypes.UpdatePostgresInput{IDOrName: "dpg-xxx"}
		err := input.Validate(false)
		assert.ErrorContains(t, err, "at least one field must be provided")
	})

	t.Run("valid with ID and a single field", func(t *testing.T) {
		input := postgrestypes.UpdatePostgresInput{
			IDOrName: "dpg-xxx",
			Name:     pointers.From("new-name"),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("rejects invalid disk size", func(t *testing.T) {
		input := postgrestypes.UpdatePostgresInput{
			IDOrName:   "dpg-xxx",
			DiskSizeGB: pointers.From(0),
		}
		err := input.Validate(false)
		assert.ErrorContains(t, err, "--disk-size-gb")
	})

	t.Run("accepts valid disk size", func(t *testing.T) {
		input := postgrestypes.UpdatePostgresInput{
			IDOrName:   "dpg-xxx",
			DiskSizeGB: pointers.From(10),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("rejects malformed parameter override", func(t *testing.T) {
		input := postgrestypes.UpdatePostgresInput{
			IDOrName:           "dpg-xxx",
			ParameterOverrides: []string{"noequals"},
		}
		err := input.Validate(false)
		assert.ErrorContains(t, err, "invalid --parameter-override")
		assert.ErrorContains(t, err, "expected KEY=VALUE format")
	})

	t.Run("accepts valid parameter override", func(t *testing.T) {
		input := postgrestypes.UpdatePostgresInput{
			IDOrName:           "dpg-xxx",
			ParameterOverrides: []string{"max_connections=100"},
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("rejects ip-allow-list and clear-ip-allow-list together", func(t *testing.T) {
		input := postgrestypes.UpdatePostgresInput{
			IDOrName:         "dpg-xxx",
			IPAllowList:      []string{"cidr=10.0.0.0/8,description=internal"},
			ClearIPAllowList: true,
		}
		err := input.Validate(false)
		assert.ErrorContains(t, err, "mutually exclusive")
	})

	t.Run("rejects malformed ip-allow-list entry", func(t *testing.T) {
		input := postgrestypes.UpdatePostgresInput{
			IDOrName:    "dpg-xxx",
			IPAllowList: []string{"not-a-cidr-entry"},
		}
		err := input.Validate(false)
		require.Error(t, err)
	})

	t.Run("accepts valid ip-allow-list entry", func(t *testing.T) {
		input := postgrestypes.UpdatePostgresInput{
			IDOrName:    "dpg-xxx",
			IPAllowList: []string{"cidr=10.0.0.0/8,description=internal"},
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("clear-ip-allow-list alone is a valid mutation", func(t *testing.T) {
		input := postgrestypes.UpdatePostgresInput{
			IDOrName:         "dpg-xxx",
			ClearIPAllowList: true,
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})
}

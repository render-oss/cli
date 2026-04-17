package postgres_test

import (
	"testing"

	postgrestypes "github.com/render-oss/cli/v2/pkg/types/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdatePostgresInputValidate(t *testing.T) {
	t.Run("requires ID or name argument", func(t *testing.T) {
		input := postgrestypes.UpdatePostgresInput{}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "postgres ID or name argument is required")
	})

	t.Run("valid with just ID", func(t *testing.T) {
		input := postgrestypes.UpdatePostgresInput{
			IDOrName: "dpg-xxx",
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
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --parameter-override")
		assert.Contains(t, err.Error(), "expected KEY=VALUE format")
	})

	t.Run("accepts valid parameter override", func(t *testing.T) {
		input := postgrestypes.UpdatePostgresInput{
			IDOrName:           "dpg-xxx",
			ParameterOverrides: []string{"max_connections=100"},
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})
}

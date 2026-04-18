package postgres_test

import (
	"testing"

	postgrestypes "github.com/render-oss/cli/v2/pkg/types/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePostgresInputValidate(t *testing.T) {
	t.Run("requires name", func(t *testing.T) {
		input := postgrestypes.CreatePostgresInput{
			Plan:    "free",
			Version: 16,
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--name is required")
	})

	t.Run("requires plan", func(t *testing.T) {
		input := postgrestypes.CreatePostgresInput{
			Name:    "test-db",
			Version: 16,
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--plan is required")
	})

	t.Run("requires version", func(t *testing.T) {
		input := postgrestypes.CreatePostgresInput{
			Name: "test-db",
			Plan: "free",
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--version is required")
	})

	t.Run("rejects version below minimum", func(t *testing.T) {
		input := postgrestypes.CreatePostgresInput{
			Name:    "test-db",
			Plan:    "free",
			Version: 10,
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --version")
	})

	t.Run("accepts known versions", func(t *testing.T) {
		for _, version := range []int{11, 14, 16, 18} {
			input := postgrestypes.CreatePostgresInput{
				Name:    "test-db",
				Plan:    "free",
				Version: version,
			}
			err := input.Validate(false)
			require.NoError(t, err)
		}
	})

	t.Run("accepts future versions above minimum", func(t *testing.T) {
		input := postgrestypes.CreatePostgresInput{
			Name:    "test-db",
			Plan:    "free",
			Version: 99,
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("rejects malformed parameter override without equals", func(t *testing.T) {
		input := postgrestypes.CreatePostgresInput{
			Name:               "test-db",
			Plan:               "free",
			Version:            16,
			ParameterOverrides: []string{"noequals"},
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --parameter-override")
		assert.Contains(t, err.Error(), "expected KEY=VALUE format")
	})

	t.Run("accepts valid parameter override", func(t *testing.T) {
		input := postgrestypes.CreatePostgresInput{
			Name:               "test-db",
			Plan:               "free",
			Version:            16,
			ParameterOverrides: []string{"max_connections=100"},
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("accepts multiple valid parameter overrides", func(t *testing.T) {
		input := postgrestypes.CreatePostgresInput{
			Name:    "test-db",
			Plan:    "free",
			Version: 16,
			ParameterOverrides: []string{
				"max_connections=100",
				"shared_buffers=256MB",
			},
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("valid minimal input", func(t *testing.T) {
		input := postgrestypes.CreatePostgresInput{
			Name:    "test-db",
			Plan:    "free",
			Version: 16,
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})
}

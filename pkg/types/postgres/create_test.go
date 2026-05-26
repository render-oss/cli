package postgres_test

import (
	"testing"

	postgrestypes "github.com/render-oss/cli/pkg/types/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePostgresInputValidate(t *testing.T) {
	// A zero-value input must pass validation so users can run
	// `render ea pg create` with no flags and get a working database.
	// Defaults are filled in client-side (name, plan, version, disk size)
	// and server-side (region, db name, etc.).
	t.Run("zero-value input is valid", func(t *testing.T) {
		require.NoError(t, postgrestypes.CreatePostgresInput{}.Validate(false))
	})

	t.Run("rejects malformed parameter override", func(t *testing.T) {
		err := postgrestypes.CreatePostgresInput{
			ParameterOverrides: []string{"noequals"},
		}.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected KEY=VALUE format")
	})

	t.Run("accepts valid parameter overrides", func(t *testing.T) {
		require.NoError(t, postgrestypes.CreatePostgresInput{
			ParameterOverrides: []string{"max_connections=100", "shared_buffers=256MB"},
		}.Validate(false))
	})
}

func TestValidateDiskSizeGB(t *testing.T) {
	t.Run("nil is valid (means let server decide)", func(t *testing.T) {
		require.NoError(t, postgrestypes.ValidateDiskSizeGB(nil))
	})

	t.Run("accepts 1 GB", func(t *testing.T) {
		size := 1
		require.NoError(t, postgrestypes.ValidateDiskSizeGB(&size))
	})

	t.Run("accepts multiples of 5", func(t *testing.T) {
		for _, size := range []int{5, 15, 100, 250, 1000} {
			s := size
			require.NoError(t, postgrestypes.ValidateDiskSizeGB(&s))
		}
	})

	t.Run("rejects 0", func(t *testing.T) {
		size := 0
		require.Error(t, postgrestypes.ValidateDiskSizeGB(&size))
	})

	t.Run("rejects non-multiples of 5", func(t *testing.T) {
		for _, size := range []int{2, 3, 4, 7, 11, 99} {
			s := size
			require.Error(t, postgrestypes.ValidateDiskSizeGB(&s), "size %d should be invalid", s)
		}
	})
}

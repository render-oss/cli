package types_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/render-oss/cli/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestParseOptional(t *testing.T) {
	upperParser := func(v string) (string, error) {
		return strings.ToUpper(v), nil
	}

	t.Run("nil value returns nil", func(t *testing.T) {
		parsed, err := types.ParseOptional(nil, upperParser)
		require.NoError(t, err)
		require.Nil(t, parsed)
	})

	t.Run("blank value returns nil", func(t *testing.T) {
		raw := "   "
		parsed, err := types.ParseOptional(&raw, upperParser)
		require.NoError(t, err)
		require.Nil(t, parsed)
	})

	t.Run("non-empty value parses", func(t *testing.T) {
		raw := "node"
		parsed, err := types.ParseOptional(&raw, upperParser)
		require.NoError(t, err)
		require.NotNil(t, parsed)
		require.Equal(t, "NODE", *parsed)
	})

	t.Run("parser error is returned", func(t *testing.T) {
		raw := "bad"
		_, err := types.ParseOptional(&raw, func(string) (string, error) {
			return "", errors.New("invalid value")
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid value")
	})
}

func TestTrimmedNonEmpty(t *testing.T) {
	t.Run("returns trimmed value when non-empty", func(t *testing.T) {
		trimmed, ok := types.TrimmedNonEmpty("  value  ")
		require.True(t, ok)
		require.Equal(t, "value", trimmed)
	})

	t.Run("returns false for whitespace-only value", func(t *testing.T) {
		trimmed, ok := types.TrimmedNonEmpty("   ")
		require.False(t, ok)
		require.Equal(t, "", trimmed)
	})
}

func TestOptionalNonZeroString(t *testing.T) {
	t.Run("returns nil for nil pointer", func(t *testing.T) {
		require.Nil(t, types.OptionalNonZeroString(nil))
	})

	t.Run("returns nil for empty string", func(t *testing.T) {
		empty := ""
		require.Nil(t, types.OptionalNonZeroString(&empty))
	})

	t.Run("returns nil for whitespace-only string", func(t *testing.T) {
		whitespace := "   "
		require.Nil(t, types.OptionalNonZeroString(&whitespace))
	})

	t.Run("returns trimmed value for non-empty string", func(t *testing.T) {
		value := "  value  "
		result := types.OptionalNonZeroString(&value)
		require.NotNil(t, result)
		require.Equal(t, "value", *result)
	})
}

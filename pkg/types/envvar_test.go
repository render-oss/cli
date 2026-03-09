package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseEnvVar(t *testing.T) {
	t.Run("parses key and value", func(t *testing.T) {
		parsed, err := ParseEnvVar("FOO=bar")
		require.NoError(t, err)
		require.Equal(t, "FOO", parsed.Key)
		require.Equal(t, "bar", parsed.Value)
	})

	t.Run("rejects missing equals", func(t *testing.T) {
		_, err := ParseEnvVar("FOO")
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected KEY=VALUE")
	})

	t.Run("rejects empty key", func(t *testing.T) {
		_, err := ParseEnvVar(" =bar")
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected KEY=VALUE")
	})

	t.Run("allows empty value", func(t *testing.T) {
		parsed, err := ParseEnvVar("FOO=")
		require.NoError(t, err)
		require.Equal(t, "FOO", parsed.Key)
		require.Equal(t, "", parsed.Value)
	})

	t.Run("trims whitespace from key and value", func(t *testing.T) {
		parsed, err := ParseEnvVar("  FOO  =  bar  ")
		require.NoError(t, err)
		require.Equal(t, "FOO", parsed.Key)
		require.Equal(t, "bar", parsed.Value)
	})

	t.Run("preserves equals signs in value", func(t *testing.T) {
		parsed, err := ParseEnvVar("DATABASE_URL=postgres://user:pass@host/db?foo=bar")
		require.NoError(t, err)
		require.Equal(t, "DATABASE_URL", parsed.Key)
		require.Equal(t, "postgres://user:pass@host/db?foo=bar", parsed.Value)
	})
}

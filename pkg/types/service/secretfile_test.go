package service_test

import (
	"testing"

	servicetypes "github.com/render-oss/cli/v2/pkg/types/service"
	"github.com/stretchr/testify/require"
)

func TestParseSecretFileRef(t *testing.T) {
	t.Run("parses valid secret file", func(t *testing.T) {
		parsed, err := servicetypes.ParseSecretFileRef("my-secret:/tmp/secret.txt")
		require.NoError(t, err)
		require.Equal(t, "my-secret", parsed.Name)
		require.Equal(t, "/tmp/secret.txt", parsed.Path)
	})

	t.Run("rejects missing colon", func(t *testing.T) {
		_, err := servicetypes.ParseSecretFileRef("invalid")
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected NAME:LOCAL_PATH")
	})

	t.Run("rejects empty path", func(t *testing.T) {
		_, err := servicetypes.ParseSecretFileRef("name:")
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected NAME:LOCAL_PATH")
	})

	t.Run("rejects empty name", func(t *testing.T) {
		_, err := servicetypes.ParseSecretFileRef(":/tmp/secret.txt")
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected NAME:LOCAL_PATH")
	})

	t.Run("trims whitespace from name and path", func(t *testing.T) {
		parsed, err := servicetypes.ParseSecretFileRef("  my-secret  :  /tmp/secret.txt  ")
		require.NoError(t, err)
		require.Equal(t, "my-secret", parsed.Name)
		require.Equal(t, "/tmp/secret.txt", parsed.Path)
	})

	t.Run("handles multiple colons by using first as delimiter", func(t *testing.T) {
		// Windows paths or URLs in path portion may contain colons
		parsed, err := servicetypes.ParseSecretFileRef("db-config:C:\\secrets\\db.txt")
		require.NoError(t, err)
		require.Equal(t, "db-config", parsed.Name)
		require.Equal(t, "C:\\secrets\\db.txt", parsed.Path)
	})
}

package service

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveSecretFileInputs(t *testing.T) {
	t.Run("returns nil for empty input", func(t *testing.T) {
		resolved, err := ResolveSecretFileInputs(nil)
		require.NoError(t, err)
		require.Nil(t, resolved)
	})

	t.Run("rejects malformed value", func(t *testing.T) {
		_, err := ResolveSecretFileInputs([]string{"invalid"})
		require.Error(t, err)
		require.Contains(t, err.Error(), `invalid --secret-file "invalid": expected NAME:LOCAL_PATH`)
	})

	t.Run("rejects empty name or path", func(t *testing.T) {
		_, err := ResolveSecretFileInputs([]string{"name:"})
		require.Error(t, err)
		require.Contains(t, err.Error(), `invalid --secret-file "name:": expected NAME:LOCAL_PATH`)
	})

	t.Run("returns read failure", func(t *testing.T) {
		_, err := ResolveSecretFileInputs([]string{"app-secret:/definitely/missing"})
		require.Error(t, err)
		require.Contains(t, err.Error(), `failed to read --secret-file "app-secret:/definitely/missing"`)
	})

	t.Run("loads content from local file", func(t *testing.T) {
		dir := t.TempDir()
		secretPath := dir + "/secret.txt"
		require.NoError(t, os.WriteFile(secretPath, []byte("top-secret"), 0o600))

		resolved, err := ResolveSecretFileInputs([]string{"app-secret:" + secretPath})
		require.NoError(t, err)
		require.Len(t, resolved, 1)
		require.Equal(t, "app-secret", resolved[0].Name)
		require.Equal(t, "top-secret", resolved[0].Content)
	})
}

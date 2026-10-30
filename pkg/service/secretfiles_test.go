package service

import (
	"os"
	"runtime"
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

	t.Run("reads the file whose name ends in whitespace, not its trimmed neighbor", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not support these literal filenames")
		}
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(dir+"/neighbor.txt", []byte("NEIGHBOR"), 0o600))
		for _, suffix := range []string{" ", "\t", "\n"} {
			require.NoError(t, os.WriteFile(dir+"/neighbor.txt"+suffix, []byte("INTENDED"+suffix), 0o600))
			resolved, err := ResolveSecretFileInputs([]string{"app-secret:" + dir + "/neighbor.txt" + suffix})
			require.NoError(t, err)
			require.Equal(t, "INTENDED"+suffix, resolved[0].Content, "suffix %q", suffix)
		}
	})

	t.Run("a missing whitespace-suffixed file is not replaced by its neighbor", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not support these literal filenames")
		}
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(dir+"/standalone.txt", []byte("NEIGHBOR"), 0o600))
		_, err := ResolveSecretFileInputs([]string{"app-secret:" + dir + "/standalone.txt "})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read --secret-file")
	})

	t.Run("keeps interior spaces and colons in the path", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not support these literal filenames")
		}
		dir := t.TempDir()
		path := dir + "/a b:c.txt"
		require.NoError(t, os.WriteFile(path, []byte("INTERIOR"), 0o600))
		resolved, err := ResolveSecretFileInputs([]string{" app-secret :" + path})
		require.NoError(t, err)
		require.Equal(t, "app-secret", resolved[0].Name)
		require.Equal(t, "INTERIOR", resolved[0].Content)
	})
}

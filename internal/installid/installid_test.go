package installid

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestResolveCreatesAndPersistsID(t *testing.T) {
	path := setConfigDir(t, t.TempDir())

	id, err := Resolve()
	require.NoError(t, err)
	requireV4UUID(t, id)

	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, id, string(contents))

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func TestResolveReusesPersistedID(t *testing.T) {
	path := setConfigDir(t, t.TempDir())
	firstID, err := Resolve()
	require.NoError(t, err)
	before, err := os.Stat(path)
	require.NoError(t, err)

	secondID, err := Resolve()
	require.NoError(t, err)
	after, err := os.Stat(path)
	require.NoError(t, err)

	require.Equal(t, firstID, secondID)
	require.True(t, os.SameFile(before, after), "valid installation ID file should not be replaced")
}

func TestResolveReplacesInvalidContents(t *testing.T) {
	testCases := map[string]string{
		"malformed":       "not-a-uuid",
		"non-v4":          "f47ac10b-58cc-1372-8567-0e02b2c3d479",
		"non-RFC variant": "f47ac10b-58cc-4372-c567-0e02b2c3d479",
	}

	for name, invalidID := range testCases {
		t.Run(name, func(t *testing.T) {
			path := setConfigDir(t, t.TempDir())
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
			require.NoError(t, os.WriteFile(path, []byte(invalidID), 0o600))

			id, err := Resolve()
			require.NoError(t, err)
			requireV4UUID(t, id)
			require.NotEqual(t, invalidID, id)

			contents, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, id, string(contents))
		})
	}
}

func TestResolveAcceptsWhitespaceWithoutRewriting(t *testing.T) {
	path := setConfigDir(t, t.TempDir())
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	want := uuid.NewString()
	contents := []byte(want + "\n")
	require.NoError(t, os.WriteFile(path, contents, 0o600))
	before, err := os.Stat(path)
	require.NoError(t, err)

	got, err := Resolve()
	require.NoError(t, err)
	after, err := os.Stat(path)
	require.NoError(t, err)

	require.Equal(t, want, got)
	require.True(t, os.SameFile(before, after), "valid installation ID file should not be replaced")
	actualContents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, contents, actualContents)
}

func TestResolveReturnsErrorForUnwritableTarget(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "regular-file")
	require.NoError(t, os.WriteFile(configDir, []byte("not a directory"), 0o600))
	setConfigDir(t, configDir)

	id, err := Resolve()
	require.Error(t, err)
	require.Empty(t, id)
}

func setConfigDir(t *testing.T, configDir string) string {
	t.Helper()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)

	return filepath.Join(configDir, "state", "installation-id.txt")
}

func requireV4UUID(t *testing.T, value string) {
	t.Helper()
	id, err := uuid.Parse(value)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(4), id.Version())
	require.Equal(t, uuid.RFC4122, id.Variant())
}

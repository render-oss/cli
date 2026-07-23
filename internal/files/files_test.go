package files

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteCreatesDirectoriesAndReplacesFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "state")
	path := filepath.Join(dir, "state.txt")
	require.NoError(t, Write(path, []byte("old")))

	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		require.Equal(t, ownerWritableDirectory, dirInfo.Mode().Perm())
	}
	before, err := os.Stat(path)
	require.NoError(t, err)

	require.NoError(t, Write(path, []byte("new")))

	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "new", string(contents))
	after, err := os.Stat(path)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		require.False(t, os.SameFile(before, after), "Write should replace the destination file")
		require.Equal(t, ownerOnlyFile, after.Mode().Perm())
	}
}

func TestWriteCleansUpTemporaryFileWhenRenameFails(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	// A regular file cannot be renamed over a directory, forcing Write to fail
	// after creating its temporary file.
	require.NoError(t, os.Mkdir(target, 0o755))

	err := Write(target, []byte("contents"))
	require.Error(t, err)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, filepath.Base(target), entries[0].Name())
}

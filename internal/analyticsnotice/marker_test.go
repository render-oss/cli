package analyticsnotice

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarker(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", dir)

	exists, err := markerExists()
	require.NoError(t, err)
	require.False(t, exists)

	require.NoError(t, writeMarker())

	exists, err = markerExists()
	require.NoError(t, err)
	require.True(t, exists)

	path := filepath.Join(dir, "state", "analytics-notice-shown")
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Zero(t, info.Size())
}

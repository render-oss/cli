package cmd

import (
	"os"
	"testing"

	"github.com/render-oss/cli/internal/testrequire"
	"github.com/stretchr/testify/require"
)

// CommandResult holds the captured output from a CLI command execution.
type CommandResult struct {
	Stdout string
	Stderr string
}

func newTestConfigPath(t *testing.T) string {
	t.Helper()
	tmpCfg, err := os.CreateTemp("", "render-test-config-*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(tmpCfg.Name()) })
	_ = tmpCfg.Close()
	return tmpCfg.Name()
}

// requireSubMap returns the nested map stored at key, failing the test if the
// key is absent or the value is not a map[string]any.
func requireSubMap(t *testing.T, body map[string]any, key string) map[string]any {
	t.Helper()
	return testrequire.SubMap(t, body, key)
}

// requireSubSlice returns the nested slice stored at key, failing the test if
// the key is absent or the value is not a []any.
func requireSubSlice(t *testing.T, body map[string]any, key string) []any {
	t.Helper()
	require.Contains(t, body, key, "expected %q", key)
	require.IsType(t, []any{}, body[key], "expected %q to contain a slice", key)
	return body[key].([]any)
}

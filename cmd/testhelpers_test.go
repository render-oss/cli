package cmd

import (
	"os"
	"testing"

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

package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkspaceSetHelpRecommendsConfigDir(t *testing.T) {
	cmd := WorkspaceSetCmd(nil)

	require.Contains(t, cmd.Long, "RENDER_CLI_CONFIG_DIR")
	require.NotContains(t, cmd.Long, "RENDER_CLI_CONFIG_PATH")
}

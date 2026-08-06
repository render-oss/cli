//go:build windows

package analytics

import (
	"os/exec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetDetachedProcessAttributes(t *testing.T) {
	cmd := exec.Command("render.exe")

	setDetachedProcessAttributes(cmd)

	require.Equal(
		t,
		uint32(syscall.CREATE_NEW_PROCESS_GROUP|detachedProcess),
		cmd.SysProcAttr.CreationFlags,
	)
	require.Empty(t, cmd.Dir,
		"detaching must not chdir the child: a relative RENDER_CLI_CONFIG_DIR resolves against the parent's cwd")
}

func TestDetachedChildOutlivesLauncherProcess(t *testing.T) {
	testDetachedChildOutlivesLauncherProcess(t)
}

//go:build darwin || freebsd || linux

package analytics

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetDetachedProcessAttributes(t *testing.T) {
	cmd := exec.Command("render")

	setDetachedProcessAttributes(cmd)

	require.True(t, cmd.SysProcAttr.Setsid)
	require.Empty(t, cmd.Dir,
		"detaching must not chdir the child: a relative RENDER_CLI_CONFIG_DIR resolves against the parent's cwd")
}

func TestDetachedChildOutlivesLauncherProcess(t *testing.T) {
	testDetachedChildOutlivesLauncherProcess(t)
}

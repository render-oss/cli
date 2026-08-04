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
}

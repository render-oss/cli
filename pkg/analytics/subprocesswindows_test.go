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
}

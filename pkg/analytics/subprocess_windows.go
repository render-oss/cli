//go:build windows

package analytics

import (
	"os/exec"
	"syscall"
)

// detachedProcess is the Windows DETACHED_PROCESS creation flag. The syscall
// package does not expose it, so define it here rather than adding a dependency
// solely for the constant.
const detachedProcess = 0x00000008

// setDetachedProcessAttributes configures cmd without attaching it to the
// parent's console and places it in a new process group. This lets the analytics
// subprocess finish after the CLI exits without console events taking it down
// with the parent. os/exec exposes these OS-specific creation controls through
// Cmd.SysProcAttr rather than providing a portable "detached process"
// abstraction.
func setDetachedProcessAttributes(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess,
	}
}

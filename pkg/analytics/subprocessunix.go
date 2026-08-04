//go:build darwin || freebsd || linux

package analytics

import (
	"os/exec"
	"syscall"
)

// setDetachedProcessAttributes configures cmd to start in a new session,
// separating it from the parent's process group and controlling terminal. This
// lets the analytics subprocess finish after the CLI exits without terminal
// closure or process-group signals taking it down with the parent. os/exec
// exposes these OS-specific creation controls through Cmd.SysProcAttr rather
// than providing a portable "detached process" abstraction.
func setDetachedProcessAttributes(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

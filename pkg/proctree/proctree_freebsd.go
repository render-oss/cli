//go:build freebsd

package proctree

import (
	"syscall"
)

func startPlatform(p *ProcTree) error {
	// Start the child in a new session (which also gives it a new process group
	// with pgid == child's pid). That lets us signal the entire tree via -pid.
	p.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	return p.cmd.Start()
}

func killPlatformTree(p *ProcTree) error {
	if p.cmd.Process == nil {
		return nil
	}
	// Signal the whole process group. Negative PID targets the PGID.
	_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	return p.cmd.Process.Kill()
}

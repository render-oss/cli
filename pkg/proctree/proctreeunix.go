//go:build darwin || linux

package proctree

import (
	"syscall"
)

func startPlatform(p *ProcTree) error {
	// Put child in its own process group so we can signal the whole group.
	p.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		// On Linux you could also set Pdeathsig: syscall.SIGKILL
		// but it's not portable to macOS, so we skip it here.
	}
	return p.cmd.Start()
}

// Force kill the whole group.
func killPlatformTree(p *ProcTree) error {
	if p.cmd.Process == nil {
		return nil
	}
	// As a fallback, also kill the root if the group kill fails.
	_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	return p.cmd.Process.Kill()
}

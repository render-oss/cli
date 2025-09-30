package proctree

import (
	"os/exec"
)

type ProcTree struct {
	cmd *exec.Cmd
	// platform-specific fields live in *_unix.go / *_windows.go
}

// New creates a ProcTree for the given command + args.
// Use Start to actually launch it.
func New(cmd *exec.Cmd) *ProcTree {
	return &ProcTree{
		cmd: cmd,
	}
}

// Start applies platform-specific setup and launches the process.
func (p *ProcTree) Start() error {
	return startPlatform(p)
}

// PID returns the root child PID (useful for logs).
func (p *ProcTree) PID() int {
	if p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// Wait waits for process exit.
func (p *ProcTree) Wait() error {
	return p.cmd.Wait()
}

func (p *ProcTree) Kill() error {
	return killPlatformTree(p)
}

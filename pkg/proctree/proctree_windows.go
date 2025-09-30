//go:build windows

package proctree

func startPlatform(p *ProcTree) error {
	return p.cmd.Start()
}

// For now on windows we just kill the process, and ignore children
func killPlatformTree(p *ProcTree) error {
	return p.cmd.Process.Kill()
}

package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newSandboxPortsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ports",
		Short: "Manage a sandbox's public ports",
		Long: `Manage the ports a sandbox exposes at public URLs.

An exposed port is reachable over HTTP and WebSockets at its own URL. A
sandbox can expose up to 10 ports. Expose ports when the sandbox starts with
"render sandboxes create --expose-ports".`,
		Example: `  # Expose port 8080
  render sandboxes ports expose sbx-abc123 8080

  # List the exposed ports and their URLs
  render sandboxes ports list sbx-abc123

  # Stop exposing port 8080
  render sandboxes ports hide sbx-abc123 8080`,
	}
	cmd.AddCommand(children...)
	return cmd
}

type SandboxPortInput struct {
	SandboxID string `cli:"arg:0"`
	Port      string `cli:"arg:1"`
}

func (i *SandboxPortInput) port() (int, error) {
	port, err := strconv.Atoi(i.Port)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q: use a number from 1 to 65535", i.Port)
	}
	return port, nil
}

package cmd

import (
	"github.com/spf13/cobra"
)

func newSandboxCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Manage sandboxes",
		Long: `Manage sandboxes for your Render workspace.

Sandboxes are ephemeral compute environments for running code, agents, and experiments.

Available commands:
  create   - Create a new sandbox
  exec     - Execute a command in a sandbox
  list     - List sandboxes
  stop     - Terminate a running sandbox

Examples:
  render ea sandbox create --base=render/sandbox-python
  render ea sandbox exec sbx-abc123 -- echo hello
  render ea sandbox list
  render ea sandbox list --all
  render ea sandbox stop trn-abc123 --confirm
`,
	}
	cmd.AddCommand(children...)
	return cmd
}

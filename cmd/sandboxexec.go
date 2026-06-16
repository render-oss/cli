package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

// shellUnsafe matches any character that is not safe to pass to a shell
// unquoted. A token containing one of these is wrapped in single quotes so the
// remote shell re-splits the command exactly as the user's local shell did.
var shellUnsafe = regexp.MustCompile(`[^\w@%+=:,./-]`)

var sandboxExecExit = os.Exit

type SandboxExecInput struct {
	SandboxID string
	Command   string
}

func (i *SandboxExecInput) Validate() error {
	if i.SandboxID == "" {
		return fmt.Errorf("sandbox ID is required")
	}
	if i.Command == "" {
		return fmt.Errorf("a command is required")
	}
	return nil
}

func newSandboxExecCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec <sandboxId> -- <command>",
		Short: "Execute a command in a sandbox",
		Long: `Run a single command in a running sandbox. Blocks until the command
exits and returns stdout, stderr, and exit code.

Pass the command after a "--" separator so its own flags aren't parsed by the
CLI.

Examples:
  render ea sandbox exec sbx-abc123 -- echo hello
  render ea sandbox exec sbx-abc123 -- python script.py
`,
		Args: cobra.MinimumNArgs(2),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		input := SandboxExecInput{
			SandboxID: args[0],
			Command:   joinShellCommand(args[1:]),
		}
		if err := input.Validate(); err != nil {
			return err
		}

		result, err := deps.SandboxService().Exec(cmd.Context(), input.SandboxID, input.Command)
		if err != nil {
			return err
		}

		if err := writeSandboxExecResult(cmd, result); err != nil {
			return err
		}
		exitSandboxExec(result.ExitCode)
		return nil
	}

	return cmd
}

// joinShellCommand reconstructs a single shell command string from the
// already-tokenized command args. Each token is shell-quoted only when it
// contains characters the shell would otherwise interpret, so a plain
// `echo hello` stays `echo hello` while `echo "a b"` becomes `echo 'a b'`.
func joinShellCommand(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellQuote(arg)
	}
	return strings.Join(quoted, " ")
}

func shellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	if !shellUnsafe.MatchString(arg) {
		return arg
	}
	// Wrap in single quotes, terminating the quote around any embedded
	// single quote: ' -> '\''
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

func exitSandboxExec(exitCode int) {
	if exitCode != 0 {
		sandboxExecExit(exitCode)
	}
}

func writeSandboxExecResult(cmd *cobra.Command, result *sandboxclient.SandboxExecSyncResponse) error {
	output := command.GetFormatFromContext(cmd.Context())
	if output != nil && (*output == command.JSON || *output == command.YAML) {
		_, err := command.PrintData(cmd, result, func(r *sandboxclient.SandboxExecSyncResponse) string {
			return r.Stdout
		})
		return err
	}

	if result.Stdout != "" {
		if _, err := fmt.Fprint(cmd.OutOrStdout(), result.Stdout); err != nil {
			return err
		}
	}
	if result.Stderr != "" {
		if _, err := fmt.Fprint(cmd.ErrOrStderr(), result.Stderr); err != nil {
			return err
		}
	}
	return nil
}

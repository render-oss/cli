package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandbox"
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
		Long: `Run a single command in a running sandbox. Streams stdout and stderr as
the command runs, then exits with the remote command's exit code.

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

		exitCode, err := deps.SandboxService().ExecStream(cmd.Context(), input.SandboxID, input.Command,
			func(output *sandbox.ExecOutputEvent) error {
				if output.Stream == sandbox.ExecOutputStreamStderr {
					_, err := fmt.Fprint(cmd.ErrOrStderr(), output.Data)
					return err
				}
				_, err := fmt.Fprint(cmd.OutOrStdout(), output.Data)
				return err
			})
		if err != nil {
			return err
		}

		exitSandboxExec(exitCode)
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

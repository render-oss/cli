package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandbox"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxSuspendInput struct {
	SandboxID string `cli:"arg:0"`
	Wait      bool   `cli:"wait"`
}

func newSandboxSuspendCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "suspend <sandboxId>",
		Short:        "Suspend a sandbox",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Suspend a running sandbox. Open commands and connections end. The next
connection, such as "render sandboxes exec", resumes the sandbox.

The command returns while the sandbox is still suspending. Pass --wait to
return once it is suspended. Suspending a sandbox that is already suspending or
suspended succeeds. Suspending a sandbox that is not running fails.`,
		Example: `  # Suspend a sandbox
  render sandboxes suspend sbx-abc123

  # Wait until the sandbox is suspended
  render sandboxes suspend sbx-abc123 --wait

  # JSON output
  render sandboxes suspend sbx-abc123 --output json`,
	}

	cmd.Flags().Bool("wait", false, fmt.Sprintf("Wait until the sandbox is suspended, up to %d seconds, instead of returning while it's still suspending", int(sandbox.WaitTimeout.Seconds())))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxSuspendInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		// Stream status updates to stderr only for text output, so JSON/YAML
		// consumers get a single clean payload on stdout.
		var onEvent func(*sandboxclient.Sandbox)
		if format := command.GetFormatFromContext(cmd.Context()); format != nil && *format == command.TEXT {
			onEvent = func(sb *sandboxclient.Sandbox) {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  %s %s\n", sb.Id, sb.Status)
			}
		}

		// A failed wait still prints the sandbox with its last status.
		var waitErr error
		_, err := command.NonInteractive(cmd, func() (*sandboxclient.Sandbox, error) {
			svc := deps.SandboxService()
			if err := svc.Suspend(cmd.Context(), input.SandboxID); err != nil {
				return nil, err
			}
			if !input.Wait {
				return svc.Get(cmd.Context(), input.SandboxID)
			}
			ctx, stop := waitContext(cmd.Context())
			defer stop()
			sb, err := svc.WaitUntilSuspended(ctx, input.SandboxID, onEvent)
			if sb == nil {
				return nil, err
			}
			waitErr = err
			return sb, nil
		}, func(sb *sandboxclient.Sandbox) string {
			return text.SandboxDetail(sb) + "\n"
		})
		if err != nil {
			return err
		}
		return waitErr
	}

	return cmd
}

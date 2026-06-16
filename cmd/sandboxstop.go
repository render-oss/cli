package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandbox"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxStopInput struct {
	SandboxID string `cli:"arg:0"`
}

func (i *SandboxStopInput) Validate(_ bool) error {
	if i.SandboxID == "" {
		return fmt.Errorf("sandbox ID is required")
	}
	return nil
}

func newSandboxStopCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "stop <sandboxId>",
		Short:        "Terminate a sandbox",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Terminate a running sandbox. This action is irreversible.

Without --confirm, this command previews what would be terminated and makes no
changes. Pass --confirm to actually terminate the sandbox.`,
		Example: `  # Preview termination (no changes made)
  render ea sandbox stop trn-abc123

  # Terminate the sandbox
  render ea sandbox stop trn-abc123 --confirm

  # JSON output
  render ea sandbox stop trn-abc123 --confirm --output json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxStopInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		confirm := command.GetConfirmFromContext(cmd.Context())

		loadData := func() (*sandbox.TerminateOut, error) {
			sb, err := deps.SandboxService().Get(cmd.Context(), input.SandboxID)
			if err != nil {
				return nil, err
			}
			out := &sandbox.TerminateOut{
				Data: sb,
				Meta: sandbox.TerminateOutMeta{Terminated: confirm},
			}
			if confirm {
				if err := deps.SandboxService().Terminate(cmd.Context(), input.SandboxID); err != nil {
					return nil, err
				}
			} else {
				out.Meta.Message = "re-run with --confirm to terminate"
			}
			return out, nil
		}

		_, err := command.NonInteractive(cmd, loadData, sandboxStopTextOutput)
		return err
	}

	return cmd
}

func sandboxStopTextOutput(r *sandbox.TerminateOut) string {
	if r.Meta.Terminated {
		return "Terminated this sandbox:\n\n" + text.SandboxDetail(r.Data) + "\n"
	}
	return "This command would terminate this sandbox:\n\n" +
		text.SandboxDetail(r.Data) +
		"\n\nRe-run with --confirm to proceed\n"
}

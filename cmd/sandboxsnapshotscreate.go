package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandboxsnapshot"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxSnapshotsCreateInput struct {
	SandboxID string `cli:"arg:0"`
	Kind      string `cli:"kind"`
}

func (i *SandboxSnapshotsCreateInput) Validate(_ bool) error {
	if i.SandboxID == "" {
		return fmt.Errorf("sandbox ID is required")
	}
	if i.Kind != "" && !sandboxesclient.SandboxSnapshotKind(i.Kind).Valid() {
		return fmt.Errorf("invalid kind %q: use %s", i.Kind, strings.Join(sandboxSnapshotKindNames(), ", "))
	}
	return nil
}

func sandboxSnapshotKindNames() []string {
	return []string{
		string(sandboxesclient.Filesystem),
		string(sandboxesclient.Runtime),
	}
}

func newSandboxSnapshotsCreateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "create <sandboxId>",
		Short:        "Snapshot a running sandbox",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Capture a snapshot of a running sandbox.

The command returns as soon as the API accepts the request, while the snapshot
status is still "creating". Poll with "render ea sandboxes snapshots get" until
the status is "available" before starting a sandbox from it.

The sandbox keeps running after the capture.`,
		Example: `  # Snapshot the filesystem (default)
  render ea sandboxes snapshots create sbx-abc123

  # Also capture memory and CPU state
  render ea sandboxes snapshots create sbx-abc123 --kind runtime

  # JSON output
  render ea sandboxes snapshots create sbx-abc123 --output json`,
	}

	cmd.Flags().String("kind", "", "Snapshot kind: filesystem (default), runtime")
	setFlagPlaceholder(cmd.Flags(), "kind", "KIND")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxSnapshotsCreateInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() (*sandboxesclient.SandboxSnapshot, error) {
			return deps.SandboxSnapshotService().Create(cmd.Context(), sandboxsnapshot.CreateInput{
				SandboxID: input.SandboxID,
				Kind:      input.Kind,
			})
		}, text.SandboxSnapshotDetail)
		return err
	}

	return cmd
}

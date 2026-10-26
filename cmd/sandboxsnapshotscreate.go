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
	Name      string `cli:"name"`
	Wait      bool   `cli:"wait"`
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
status is still "creating". Pass --wait to return once it's "available", so a
sandbox can start from it.

The sandbox keeps running after the capture.`,
		Example: `  # Snapshot the filesystem, memory, and CPU state (default)
  render ea sandboxes snapshots create sbx-abc123

  # Snapshot only the filesystem
  render ea sandboxes snapshots create sbx-abc123 --kind filesystem

  # Name the snapshot
  render ea sandboxes snapshots create sbx-abc123 --name gold

  # Wait until new sandboxes can start from it
  render ea sandboxes snapshots create sbx-abc123 --name gold --wait

  # JSON output
  render ea sandboxes snapshots create sbx-abc123 --output json`,
	}

	cmd.Flags().String("kind", "", "Snapshot kind: runtime (default), filesystem")
	setFlagPlaceholder(cmd.Flags(), "kind", "KIND")
	cmd.Flags().String("name", "", "Name the snapshot")
	setFlagPlaceholder(cmd.Flags(), "name", "NAME")
	cmd.Flags().Bool("wait", false, fmt.Sprintf("Wait until the snapshot is available, up to %d minutes", int(sandboxsnapshot.WaitTimeout.Minutes())))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxSnapshotsCreateInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		var name *string
		if cmd.Flags().Changed("name") {
			name = &input.Name
		}

		// A failed wait still prints the snapshot, so callers get its ID.
		var waitErr error
		_, err := command.NonInteractive(cmd, func() (*sandboxesclient.SandboxSnapshot, error) {
			snapshot, err := deps.SandboxSnapshotService().Create(cmd.Context(), sandboxsnapshot.CreateInput{
				SandboxID: input.SandboxID,
				Kind:      input.Kind,
				Name:      name,
			})
			if err != nil || !input.Wait {
				return snapshot, err
			}
			ctx, stop := waitContext(cmd.Context())
			defer stop()
			var waited *sandboxesclient.SandboxSnapshot
			waited, waitErr = deps.SandboxSnapshotService().WaitUntilAvailable(ctx, snapshot.SandboxGroupId, snapshot.Id)
			if waited != nil {
				snapshot = waited
			}
			return snapshot, nil
		}, text.SandboxSnapshotDetail)
		if err != nil {
			return err
		}
		return waitErr
	}

	return cmd
}

package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandboxsnapshot"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxSnapshotsDeleteInput struct {
	SnapshotID string `cli:"arg:0"`
	Group      string `cli:"group"`
}

func newSandboxSnapshotsDeleteCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete <snapshotId>",
		Short:        "Delete a sandbox snapshot",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Delete one sandbox snapshot by ID. This action is irreversible.

Uses the active workspace's default sandbox group unless --group is provided.
Without --confirm, this command previews what would be deleted and makes no
changes. A snapshot that is still being created cannot be deleted yet. An
expired snapshot is reported as not found.`,
		Example: `  # Preview deletion (no changes made)
  render ea sandboxes snapshots delete snp-abc123

  # Delete the snapshot
  render ea sandboxes snapshots delete snp-abc123 --confirm

  # Delete a snapshot in a specific group
  render ea sandboxes snapshots delete snp-abc123 --group sbg-abc123 --confirm

  # JSON output
  render ea sandboxes snapshots delete snp-abc123 --confirm --output json`,
	}

	cmd.Flags().String("group", "", "Sandbox group the snapshot belongs to (defaults to the active workspace's default group)")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxSnapshotsDeleteInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		confirm := command.GetConfirmFromContext(cmd.Context())

		loadData := func() (*sandboxsnapshot.DeleteOut, error) {
			return deleteSandboxSnapshot(cmd.Context(), deps.SandboxSnapshotService(), input, confirm)
		}
		_, err := command.NonInteractive(cmd, loadData, sandboxSnapshotsDeleteTextOutput)
		return err
	}

	return cmd
}

func deleteSandboxSnapshot(ctx context.Context, svc *sandboxsnapshot.Service, input SandboxSnapshotsDeleteInput, confirm bool) (*sandboxsnapshot.DeleteOut, error) {
	snapshot, err := svc.Get(ctx, input.Group, input.SnapshotID)
	if err != nil {
		return nil, err
	}
	if !confirm {
		return &sandboxsnapshot.DeleteOut{
			Data: snapshot,
			Meta: sandboxsnapshot.DeleteOutMeta{Message: "re-run with --confirm to delete"},
		}, nil
	}
	if err := svc.Delete(ctx, snapshot.SandboxGroupId, snapshot.Id); err != nil {
		return nil, err
	}
	return &sandboxsnapshot.DeleteOut{
		Data: snapshot,
		Meta: sandboxsnapshot.DeleteOutMeta{Deleted: true},
	}, nil
}

func sandboxSnapshotsDeleteTextOutput(r *sandboxsnapshot.DeleteOut) string {
	if r.Meta.Deleted {
		return "Deleted this snapshot:\n\n" + text.SandboxSnapshotDetail(r.Data) + "\n"
	}
	return "This command would delete this snapshot:\n\n" +
		text.SandboxSnapshotDetail(r.Data) +
		"\n\nRe-run with --confirm to proceed\n"
}

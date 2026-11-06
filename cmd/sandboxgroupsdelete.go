package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandboxgroup"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxGroupsDeleteInput struct {
	SandboxGroupID string `cli:"arg:0"`
}

func newSandboxGroupsDeleteCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete <sandboxGroupId>",
		Short:        "Delete a sandbox group",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Delete a sandbox group. This action is irreversible.

Deleting a group terminates its sandboxes and deletes its snapshots and
credentials shortly after. The workspace's default group and suspended groups
cannot be deleted. Without --confirm, show the group that would be deleted
without making changes.`,
		Example: `  # Preview group deletion (no changes made)
  render sandbox-groups delete sbg-abc123

  # Delete the group
  render sandbox-groups delete sbg-abc123 --confirm

  # Output as JSON
  render sandbox-groups delete sbg-abc123 --confirm -o json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxGroupsDeleteInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		confirm := command.GetConfirmFromContext(cmd.Context())

		loadData := func() (*sandboxgroup.DeleteOut, error) {
			svc := deps.SandboxGroupService()
			return deleteSandboxGroup(cmd.Context(), svc, input.SandboxGroupID, confirm)
		}
		_, err := command.NonInteractive(cmd, loadData, sandboxGroupsDeleteTextOutput)
		return err
	}
	return cmd
}

func deleteSandboxGroup(
	ctx context.Context, svc *sandboxgroup.Service, id string, confirm bool,
) (*sandboxgroup.DeleteOut, error) {
	group, err := svc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if group.IsDefault {
		return nil, sandboxgroup.ErrDefaultGroupNotDeletable
	}
	if !confirm {
		return &sandboxgroup.DeleteOut{
			Data: group,
			Meta: sandboxgroup.DeleteOutMeta{Message: "re-run with --confirm to delete"},
		}, nil
	}
	if err := svc.Delete(ctx, group.Id); err != nil {
		return nil, err
	}
	return &sandboxgroup.DeleteOut{Data: group, Meta: sandboxgroup.DeleteOutMeta{Deleted: true}}, nil
}

func sandboxGroupsDeleteTextOutput(out *sandboxgroup.DeleteOut) string {
	if out.Meta.Deleted {
		return fmt.Sprintf("Deleted sandbox group %s\n", out.Data.Id)
	}
	return "This command would delete this sandbox group:\n\n" +
		text.SandboxGroupDetail(out.Data) +
		"\nDeleting it terminates its sandboxes and deletes its snapshots and credentials." +
		"\n\nRe-run with --confirm to proceed\n"
}

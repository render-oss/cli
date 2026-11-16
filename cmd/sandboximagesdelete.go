package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandboximage"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxImagesDeleteInput struct {
	ImageID string `cli:"arg:0"`
	Group   string `cli:"group"`
}

func newSandboxImagesDeleteCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete <imageId>",
		Short:        "Delete a sandbox image",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Delete one sandbox image by ID. This action is irreversible.

Uses the active workspace's default sandbox group unless --group is provided.
Without --confirm, this command previews what would be deleted and makes no
changes.

An image that sandboxes or snapshots still use can't be deleted. The error
names them; terminate the sandboxes and delete the snapshots, then retry.`,
		Example: `  # Preview deletion (no changes made)
  render sandbox-groups images delete sbb-abc123

  # Delete the image
  render sandbox-groups images delete sbb-abc123 --confirm

  # Delete an image in a specific group
  render sandbox-groups images delete sbb-abc123 --group sbg-abc123 --confirm

  # JSON output
  render sandbox-groups images delete sbb-abc123 --confirm --output json`,
	}

	cmd.Flags().String("group", "", "Sandbox group the image belongs to (defaults to the active workspace's default group)")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxImagesDeleteInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		confirm := command.GetConfirmFromContext(cmd.Context())

		loadData := func() (*sandboximage.DeleteOut, error) {
			return deleteSandboxImage(cmd.Context(), deps.SandboxImageService(), input, confirm)
		}
		_, err := command.NonInteractive(cmd, loadData, sandboxImagesDeleteTextOutput)
		return err
	}

	return cmd
}

func deleteSandboxImage(ctx context.Context, svc *sandboximage.Service, input SandboxImagesDeleteInput, confirm bool) (*sandboximage.DeleteOut, error) {
	image, err := svc.Get(ctx, input.Group, input.ImageID)
	if err != nil {
		return nil, err
	}
	if !confirm {
		return &sandboximage.DeleteOut{
			Data: image,
			Meta: sandboximage.DeleteOutMeta{Message: "re-run with --confirm to delete"},
		}, nil
	}
	if err := svc.Delete(ctx, image.SandboxGroupId, image.Id); err != nil {
		return nil, err
	}
	return &sandboximage.DeleteOut{
		Data: image,
		Meta: sandboximage.DeleteOutMeta{Deleted: true},
	}, nil
}

func sandboxImagesDeleteTextOutput(r *sandboximage.DeleteOut) string {
	if r.Meta.Deleted {
		return "Deleted this image:\n\n" + text.SandboxImageDetail(r.Data) + "\n"
	}
	return "This command would delete this image:\n\n" +
		text.SandboxImageDetail(r.Data) +
		"\n\nRe-run with --confirm to proceed\n"
}

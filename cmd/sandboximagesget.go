package cmd

import (
	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxImagesGetInput struct {
	ImageID string `cli:"arg:0"`
	Group   string `cli:"group"`
}

func newSandboxImagesGetCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get <imageId>",
		Short:        "Get a sandbox image",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Get one sandbox image by ID.

Uses the active workspace's default sandbox group unless --group is provided.
An image that belongs to another sandbox group is reported as not found.`,
		Example: `  # Get an image
  render sandbox-groups images get sbb-abc123

  # Get an image in a specific group
  render sandbox-groups images get sbb-abc123 --group sbg-abc123

  # JSON output
  render sandbox-groups images get sbb-abc123 --output json`,
	}

	cmd.Flags().String("group", "", "Sandbox group the image belongs to (defaults to the active workspace's default group)")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxImagesGetInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() (*sandboxesclient.SandboxBuild, error) {
			return deps.SandboxImageService().Get(cmd.Context(), input.Group, input.ImageID)
		}, text.SandboxImageDetail)
		return err
	}

	return cmd
}

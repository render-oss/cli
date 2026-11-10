package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandboximage"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxImagesListInput struct {
	Group  string   `cli:"group"`
	Name   string   `cli:"name"`
	Status []string `cli:"status"`
}

func (i *SandboxImagesListInput) Validate(_ bool) error {
	for _, status := range i.Status {
		if !sandboxesclient.SandboxBuildStatus(status).Valid() {
			return fmt.Errorf("invalid status %q: use %s", status, strings.Join(sandboxImageStatusNames(), ", "))
		}
	}
	return nil
}

func sandboxImageStatusNames() []string {
	return []string{
		string(sandboxesclient.SandboxBuildStatusPending),
		string(sandboxesclient.SandboxBuildStatusRunning),
		string(sandboxesclient.SandboxBuildStatusSucceeded),
		string(sandboxesclient.SandboxBuildStatusFailed),
	}
}

func newSandboxImagesListCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List sandbox images",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		Long: `List the images in a sandbox group, newest first.

Uses the active workspace's default sandbox group unless --group is provided.
Each import is listed separately, so an image name can appear more than once.
"render sandboxes create --image NAME" starts from the newest succeeded
image with that name.

Run "render sandbox-groups images get IMAGE_ID" to see why an import failed.`,
		Example: `  # List every image in the default sandbox group
  render sandbox-groups images list

  # List images in a specific group
  render sandbox-groups images list --group sbg-abc123

  # Every import of the image named python
  render sandbox-groups images list --name python

  # Only images sandboxes can start from
  render sandbox-groups images list --status succeeded

  # JSON output
  render sandbox-groups images list --output json`,
	}

	cmd.Flags().String("group", "", "Sandbox group to list (defaults to the active workspace's default group)")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")
	cmd.Flags().String("name", "", "Only list images with exactly this name")
	setFlagPlaceholder(cmd.Flags(), "name", "NAME")
	cmd.Flags().StringArray("status", nil, "Filter by status (repeatable: "+strings.Join(sandboxImageStatusNames(), ", ")+")")
	setFlagPlaceholder(cmd.Flags(), "status", "STATUS")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxImagesListInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() ([]*sandboxesclient.SandboxBuild, error) {
			return deps.SandboxImageService().List(cmd.Context(), sandboximage.ListInput{
				SandboxGroupID: input.Group,
				Name:           input.Name,
				Statuses:       input.Status,
			})
		}, text.SandboxImageTable)
		return err
	}

	return cmd
}

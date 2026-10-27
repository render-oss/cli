package cmd

import (
	"github.com/spf13/cobra"
)

func newSandboxImagesCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "images",
		Short: "Manage sandbox images",
		Long: `Manage the images sandboxes start from in your Render workspace.

Importing an OCI image, such as python:3.13, prepares it so new sandboxes can
start from it with "render sandboxes create --image".

Images belong to a sandbox group. Importing or getting an image uses the active
workspace's default group unless --group is provided.

Images must be linux/amd64 and ship a shell. Alpine and Debian-based images
work; scratch and distroless images are rejected.`,
		Example: `  # Import an image and wait until sandboxes can start from it
  render sandbox-groups images import python:3.13 --name python --wait

  # Pull the image from a private registry
  echo "$REGISTRY_TOKEN" | render sandbox-groups images import ghcr.io/acme/agent:1.0 --name agent --registry-username bot --registry-password-stdin

  # Check whether an import has succeeded
  render sandbox-groups images get sbb-abc123

  # Start a sandbox from the newest succeeded image named python
  render sandboxes create --image python`,
	}
	cmd.AddCommand(children...)
	return cmd
}

// newDeprecatedSandboxBuildsCmd keeps "render sandbox-groups builds" working
// from before images were renamed. It is hidden from help. It takes fresh
// images import and get commands and changes their Use, Hidden, and
// Deprecated fields, so don't pass the ones registered under images.
func newDeprecatedSandboxBuildsCmd(importCmd, getCmd *cobra.Command) *cobra.Command {
	importCmd.Use = "create <sourceImage>"
	importCmd.Hidden = true
	importCmd.Deprecated = "use `render sandbox-groups images import` instead."
	getCmd.Use = "get <buildId>"
	getCmd.Hidden = true
	getCmd.Deprecated = "use `render sandbox-groups images get` instead."

	cmd := &cobra.Command{
		Use:        "builds",
		Short:      "Manage sandbox builds",
		Hidden:     true,
		Deprecated: "use `render sandbox-groups images` instead.",
	}
	cmd.AddCommand(importCmd, getCmd)
	return cmd
}

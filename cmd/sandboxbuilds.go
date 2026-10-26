package cmd

import (
	"github.com/spf13/cobra"
)

func newSandboxBuildsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "builds",
		Short: "Manage sandbox builds",
		Long: `Manage builds of sandbox base images in your Render workspace.

A build converts an OCI image, such as python:3.13, into a base image that new
sandboxes can start from with "render ea sandboxes create --build".

Builds belong to a sandbox group. Creating or getting a build uses the active
workspace's default group unless --group is provided.

Images must be linux/amd64 and ship a shell. Alpine and Debian-based images
work; scratch and distroless images are rejected.`,
		Example: `  # Convert an image into a sandbox base image
  render ea sandbox-groups builds create python:3.13 --name python

  # Pull the image from a private registry
  echo "$REGISTRY_TOKEN" | render ea sandbox-groups builds create ghcr.io/acme/agent:1.0 --name agent --registry-username bot --registry-password-stdin

  # Check whether a build has succeeded
  render ea sandbox-groups builds get sbb-abc123

  # Start a sandbox from the newest succeeded build named python
  render ea sandboxes create --build python`,
	}
	cmd.AddCommand(children...)
	return cmd
}

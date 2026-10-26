package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandboxbuild"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxBuildsCreateInput struct {
	SourceImage           string `cli:"arg:0"`
	Name                  string `cli:"name"`
	Group                 string `cli:"group"`
	RegistryUsername      string `cli:"registry-username"`
	RegistryPasswordStdin bool   `cli:"registry-password-stdin"`
}

func (i *SandboxBuildsCreateInput) Validate(_ bool) error {
	if i.SourceImage == "" {
		return fmt.Errorf("source image is required")
	}
	// MarkFlagsRequiredTogether only checks that both flags were set; an empty value passes it.
	if (i.RegistryUsername != "") != i.RegistryPasswordStdin {
		return fmt.Errorf("--registry-username and --registry-password-stdin must be used together")
	}
	return nil
}

func newSandboxBuildsCreateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "create <sourceImage>",
		Short:        "Convert an image into a sandbox base image",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Convert an OCI image into a sandbox base image.

The command returns as soon as the build has started, while its status is
"running". Poll with "render ea sandbox-groups builds get" until the status is
"succeeded", then start sandboxes from it with "render ea sandboxes create
--build". A failed build reports why in its failure reason.

Every build of an image shares its --name. A request for a name and image
digest the group already has, or is building, returns that build instead of
starting another, so a build by tag is rebuilt only when the tag has moved.

Uses the active workspace's default sandbox group unless --group is provided.
A registry credential is used for this build only and never stored.`,
		Example: `  # Build a public image
  render ea sandbox-groups builds create python:3.13 --name python

  # Build an image at a digest
  render ea sandbox-groups builds create python@sha256:c19d4a1... --name python

  # Pull the image from a private registry
  echo "$REGISTRY_TOKEN" | render ea sandbox-groups builds create ghcr.io/acme/agent:1.0 --name agent --registry-username bot --registry-password-stdin

  # Build in a specific group
  render ea sandbox-groups builds create python:3.13 --name python --group sbg-abc123

  # JSON output
  render ea sandbox-groups builds create python:3.13 --name python --output json`,
	}

	cmd.Flags().String("name", "", "Name of the image the build belongs to. Every build of an image shares it")
	setFlagPlaceholder(cmd.Flags(), "name", "NAME")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().String("group", "", "Sandbox group to build in (defaults to the active workspace's default group)")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")
	cmd.Flags().String("registry-username", "", "Username for pulling the image from a private registry")
	setFlagPlaceholder(cmd.Flags(), "registry-username", "USERNAME")
	cmd.Flags().Bool("registry-password-stdin", false, "Read the private registry password or token from stdin, or prompt for it in a terminal. Used for this build only and never stored")
	cmd.MarkFlagsRequiredTogether("registry-username", "registry-password-stdin")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxBuildsCreateInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		var registryPassword string
		if input.RegistryPasswordStdin {
			var err error
			if registryPassword, err = readRegistryPassword(cmd); err != nil {
				return err
			}
			if registryPassword == "" {
				return fmt.Errorf("--registry-password-stdin received an empty password")
			}
		}

		_, err := command.NonInteractive(cmd, func() (*sandboxesclient.SandboxBuild, error) {
			return deps.SandboxBuildService().Create(cmd.Context(), sandboxbuild.CreateInput{
				SandboxGroupID:   input.Group,
				Name:             input.Name,
				SourceImage:      input.SourceImage,
				RegistryUsername: input.RegistryUsername,
				RegistryPassword: registryPassword,
			})
		}, text.SandboxBuildDetail)
		return err
	}

	return cmd
}

func readRegistryPassword(cmd *cobra.Command) (string, error) {
	in := cmd.InOrStdin()
	if f, ok := in.(*os.File); ok && isatty.IsTerminal(f.Fd()) {
		var password string
		err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Registry password").EchoMode(huh.EchoModePassword).Value(&password),
		)).WithInput(f).WithOutput(cmd.ErrOrStderr()).RunWithContext(cmd.Context())
		return password, err
	}

	b, err := io.ReadAll(in)
	if err != nil {
		return "", fmt.Errorf("failed to read registry password from stdin: %w", err)
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}

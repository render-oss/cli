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
	"github.com/render-oss/cli/pkg/sandboximage"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxImagesImportInput struct {
	Source                string `cli:"arg:0"`
	Name                  string `cli:"name"`
	Group                 string `cli:"group"`
	RegistryUsername      string `cli:"registry-username"`
	RegistryPasswordStdin bool   `cli:"registry-password-stdin"`
	Wait                  bool   `cli:"wait"`
}

func (i *SandboxImagesImportInput) Validate(_ bool) error {
	if i.Source == "" {
		return fmt.Errorf("source image is required")
	}
	// MarkFlagsRequiredTogether only checks that both flags were set; an empty value passes it.
	if (i.RegistryUsername != "") != i.RegistryPasswordStdin {
		return fmt.Errorf("--registry-username and --registry-password-stdin must be used together")
	}
	return nil
}

func newSandboxImagesImportCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "import <source>",
		Short:        "Import an image so sandboxes can start from it",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Import an OCI image so sandboxes can start from it.

The command returns as soon as the import has started, while its status is
"running". Pass --wait to return once it has "succeeded", or poll with
"render sandbox-groups images get". Then start sandboxes from it with
"render sandboxes create --image". A failed import reports why in its failure
reason.

Every version of an image shares its --name. A request for a name and image
digest the group already has, or is importing, returns that image instead of
starting another, so an image by tag is imported again only when the tag has
moved.

Uses the active workspace's default sandbox group unless --group is provided.
Registry credentials are used for this import only and never stored.`,
		Example: `  # Import a public image
  render sandbox-groups images import python:3.13 --name python

  # Wait until sandboxes can start from it
  render sandbox-groups images import python:3.13 --name python --wait

  # Import an image at a digest
  render sandbox-groups images import python@sha256:c19d4a1... --name python

  # Pull the image from a private registry
  echo "$REGISTRY_TOKEN" | render sandbox-groups images import ghcr.io/acme/agent:1.0 --name agent --registry-username bot --registry-password-stdin

  # Import into a specific group
  render sandbox-groups images import python:3.13 --name python --group sbg-abc123

  # JSON output
  render sandbox-groups images import python:3.13 --name python --output json`,
	}

	cmd.Flags().String("name", "", "Name of the image. Every version of an image shares it")
	setFlagPlaceholder(cmd.Flags(), "name", "NAME")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().String("group", "", "Sandbox group to import into (defaults to the active workspace's default group)")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")
	cmd.Flags().String("registry-username", "", "Username for pulling the image from a private registry")
	setFlagPlaceholder(cmd.Flags(), "registry-username", "USERNAME")
	cmd.Flags().Bool("registry-password-stdin", false, "Read the private registry password or token from stdin, or prompt for it in a terminal. Used for this import only and never stored")
	cmd.MarkFlagsRequiredTogether("registry-username", "registry-password-stdin")
	cmd.Flags().Bool("wait", false, fmt.Sprintf("Wait until the image has succeeded, up to %d minutes", int(sandboximage.WaitTimeout.Minutes())))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxImagesImportInput
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

		// A failed wait still prints the image, so callers get its ID.
		var waitErr error
		_, err := command.NonInteractive(cmd, func() (*sandboxesclient.SandboxBuild, error) {
			image, err := deps.SandboxImageService().Import(cmd.Context(), sandboximage.ImportInput{
				SandboxGroupID:   input.Group,
				Name:             input.Name,
				Source:           input.Source,
				RegistryUsername: input.RegistryUsername,
				RegistryPassword: registryPassword,
			})
			if err != nil || !input.Wait {
				return image, err
			}
			ctx, stop := waitContext(cmd.Context())
			defer stop()
			var waited *sandboxesclient.SandboxBuild
			waited, waitErr = deps.SandboxImageService().WaitUntilReady(ctx, image.SandboxGroupId, image.Id)
			if waited != nil {
				image = waited
			}
			return image, nil
		}, text.SandboxImageDetail)
		if err != nil {
			return err
		}
		return waitErr
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

package cmd

import (
	"context"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/flows"
	"github.com/render-oss/cli/pkg/tui/views"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/resource"
	"github.com/render-oss/cli/pkg/validate"
)

func newSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [serviceID|serviceName|instanceID]",
		Short: "SSH into a service instance",
		Long: `SSH into a service instance.

Specify a service ID, service name, or instance ID. Put SSH arguments after --:
  render ssh [serviceID|serviceName|instanceID] -- [ssh args]

For non-interactive use, specify a service or instance and set --output text. With a service, SSH connects to any available instance. The command streams SSH output without formatting. The json and yaml output formats stream the same raw output as text.

SSH uses your local SSH keys. Add the public key to your Render account before you connect. See https://render.com/docs/ssh#ssh-setup.`,
		GroupID: GroupSession.ID,
		Example: `  # Open an SSH session for a service
  render ssh srv-abc123

  # Connect to an ephemeral instance
  render ssh srv-abc123 --ephemeral

  # Connect to an ephemeral instance with a specific plan
  render ssh srv-abc123 --ephemeral --plan 1c-2g

  # Pass through ssh arguments
  render ssh srv-abc123 -- -L 5432:localhost:5432

  # Run a command without an instance menu
  render ssh srv-abc123 --output text -- -o BatchMode=yes 'echo hello'`,
		RunE: runSSH,
	}
	cmd.Flags().BoolP("ephemeral", "e", false, "Connect to an ephemeral instance")
	cmd.Flags().String("plan", "", "Plan name to use for the ephemeral instance (e.g. 0.5c-512mb, 1c-2g, 2c-4g). Only valid with --ephemeral.")
	return cmd
}

func setupSSHCommands(root *cobra.Command, deps *dependencies.Dependencies) {
	deps.SSH.SSHCmd = newSSHCmd()
	root.AddCommand(deps.SSH.SSHCmd)
}

// InteractiveSSHView opens the SSH view for input. Other commands open it from
// their palettes, so it reads the ssh command from the dependencies in ctx.
var InteractiveSSHView = func(ctx context.Context, input *views.SSHInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, dependencies.GetFromContext(ctx).SSHCmd(), breadcrumb, input, views.NewSSHView(ctx, input))
}

func interactiveSSHView(ctx context.Context, cmd *cobra.Command, input *views.SSHInput, breadcrumb string) tea.Cmd {
	if input.InstanceID != "" {
		return InteractiveSSHView(ctx, input, breadcrumb)
	}
	if input.ServiceIDOrName == "" {
		// No service specified, show service selection
		return command.AddToStackFunc(
			ctx,
			cmd,
			breadcrumb,
			input,
			views.NewServiceList(ctx, getServiceListInput(ctx, input), func(ctx context.Context, r resource.Resource) tea.Cmd {
				input.ServiceIDOrName = r.ID()
				if input.Ephemeral {
					return InteractiveSSHView(ctx, input, "SSH")
				}
				// Show instance selection for the selected service
				return command.AddToStackFunc(
					ctx,
					cmd,
					"Select Instance",
					input,
					views.NewSSHInstanceSelectionView(ctx, r.ID(), func(instanceID string) tea.Cmd {
						input.InstanceID = instanceID
						return InteractiveSSHView(ctx, input, "SSH")
					}),
				)
			}, tui.WithCustomOptions[*service.Model](getSSHTableOptions(ctx, breadcrumb))),
		)
	} else if validate.IsServiceID(input.ServiceIDOrName) {
		if input.Ephemeral {
			return InteractiveSSHView(ctx, input, "SSH")
		}
		// Service ID provided, show instance selection
		return command.AddToStackFunc(
			ctx,
			cmd,
			"Select Instance",
			input,
			views.NewSSHInstanceSelectionView(ctx, input.ServiceIDOrName, func(instanceID string) tea.Cmd {
				input.InstanceID = instanceID
				return InteractiveSSHView(ctx, input, breadcrumb)
			}),
		)
	}

	return InteractiveSSHView(ctx, input, breadcrumb)
}

func getServiceListInput(ctx context.Context, input *views.SSHInput) views.ServiceInput {
	serviceListInput := views.ServiceInput{
		Project:        input.Project,
		EnvironmentIDs: input.EnvironmentIDs,
		Types:          []client.ServiceType{client.WebService, client.PrivateService, client.BackgroundWorker},
	}

	if len(input.EnvironmentIDs) == 0 {
		if defaultInput, err := views.DefaultListResourceInput(ctx); err == nil {
			serviceListInput.Project = defaultInput.Project
			serviceListInput.EnvironmentIDs = defaultInput.EnvironmentIDs
		}
	}

	return serviceListInput
}

func getSSHTableOptions(ctx context.Context, breadcrumb string) []tui.CustomOption {
	return []tui.CustomOption{
		flows.WithCopyID(ctx, servicesCmd),
		flows.WithWorkspaceSelection(ctx),
		flows.WithProjectFilter(ctx, servicesCmd, "Project Filter", &views.SSHInput{}, func(ctx context.Context, project *client.Project) tea.Cmd {
			input := views.SSHInput{}
			if project != nil {
				input.Project = project
				input.EnvironmentIDs = project.EnvironmentIds
			}
			// A project selection returns to the service chooser; no service has been chosen yet.
			return interactiveSSHView(ctx, dependencies.GetFromContext(ctx).SSHCmd(), &input, breadcrumb)
		}),
	}
}

func runSSH(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	input := views.SSHInput{}
	if dash := cmd.ArgsLenAtDash(); dash >= 0 {
		input.Args = args[dash:]
		args = args[:dash]
	}
	if len(args) > 1 {
		return fmt.Errorf("unexpected argument %q: put SSH arguments after --", args[1])
	}
	if err := command.ParseCommand(cmd, args, &input); err != nil {
		return err
	}

	if validate.IsServiceInstanceID(input.ServiceIDOrName) {
		input.InstanceID = input.ServiceIDOrName
		input.ServiceIDOrName = validate.ExtractServiceIDFromInstanceID(input.ServiceIDOrName)
	}
	if command.IsInteractive(ctx) {
		interactiveSSHView(ctx, cmd, &input, "SSH")
		return nil
	}
	if input.ServiceIDOrName == "" {
		return fmt.Errorf("service ID, service name, or instance ID is required in non-interactive mode")
	}

	process, err := views.SSHCommand(ctx, &input)
	if err != nil {
		return err
	}
	process.Stdin = cmd.InOrStdin()
	process.Stdout = cmd.OutOrStdout()
	process.Stderr = cmd.ErrOrStderr()
	if err := process.Run(); err != nil {
		if exitCode, ok := processExitCode(err); ok {
			return exitWithProcessCode(cmd, exitCode)
		}
		return err
	}
	return nil
}

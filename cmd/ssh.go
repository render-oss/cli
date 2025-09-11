package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/resource"
	"github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
	"github.com/render-oss/cli/pkg/validate"
)

// sshCmd represents the ssh command
var sshCmd = &cobra.Command{
	Use:   "ssh [serviceID|serviceName|instanceID]",
	Short: "SSH into a service instance",
	Long: `SSH into a service instance. You can specify the service ID, service name, or specific instance ID as an argument.

To pass arguments to ssh, use the following syntax: render ssh [serviceID|serviceName|instanceID] -- [ssh args]`,
	GroupID: GroupSession.ID,
}

var InteractiveSSHView = func(ctx context.Context, input *views.SSHInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, sshCmd, breadcrumb, input, views.NewSSHView(ctx, input))
}

func interactiveSSHView(ctx context.Context, input *views.SSHInput, breadcrumb string) tea.Cmd {
	if validate.IsServiceInstanceID(input.ServiceIDOrName) {
		// Instance ID provided, extract service ID and go directly to SSH
		input.InstanceID = input.ServiceIDOrName
		input.ServiceIDOrName = validate.ExtractServiceIDFromInstanceID(input.ServiceIDOrName)
		return InteractiveSSHView(ctx, input, breadcrumb)
	}

	if input.ServiceIDOrName == "" {
		// No service specified, show service selection
		return command.AddToStackFunc(
			ctx,
			sshCmd,
			breadcrumb,
			input,
			views.NewServiceList(ctx, getServiceListInput(ctx, input), func(ctx context.Context, r resource.Resource) tea.Cmd {
				input.ServiceIDOrName = r.ID()

				// Show instance selection for the selected service
				return command.AddToStackFunc(
					ctx,
					sshCmd,
					"Select Instance",
					input,
					views.NewSSHInstanceSelectionView(ctx, r.ID(), func(instanceID string) tea.Cmd {
						input.InstanceID = input.ServiceIDOrName
						return InteractiveSSHView(ctx, input, "SSH")
					}),
				)
			}, tui.WithCustomOptions[*service.Model](getSSHTableOptions(ctx, breadcrumb))),
		)
	} else if validate.IsServiceID(input.ServiceIDOrName) {
		// Service ID provided, show instance selection
		return command.AddToStackFunc(
			ctx,
			sshCmd,
			"Select Instance",
			input,
			views.NewSSHInstanceSelectionView(ctx, input.ServiceIDOrName, func(instanceID string) tea.Cmd {
				input.InstanceID = input.ServiceIDOrName
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
		WithCopyID(ctx, servicesCmd),
		WithWorkspaceSelection(ctx),
		WithProjectFilter(ctx, servicesCmd, "Project Filter", &views.SSHInput{}, func(ctx context.Context, project *client.Project) tea.Cmd {
			input := views.SSHInput{}
			if project != nil {
				input.Project = project
				input.EnvironmentIDs = project.EnvironmentIds
			}
			return InteractiveSSHView(ctx, &input, breadcrumb)
		}),
	}
}

func init() {
	rootCmd.AddCommand(sshCmd)

	sshCmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		input := views.SSHInput{}
		err := command.ParseCommandInteractiveOnly(cmd, args, &input)
		if err != nil {
			return err
		}

		if cmd.ArgsLenAtDash() == 0 {
			input.ServiceIDOrName = ""
		}

		if cmd.ArgsLenAtDash() >= 0 {
			input.Args = args[cmd.ArgsLenAtDash():]
		}

		interactiveSSHView(ctx, &input, "SSH")
		return nil
	}
}

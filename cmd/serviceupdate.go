package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
)

var ServiceUpdateCmd = &cobra.Command{
	Use:   "update [service]",
	Args:  cobra.ExactArgs(1),
	Short: "Update a service",
	Long: `Update a service on Render.

This command currently runs in non-interactive mode only.
Provide all updates with flags.

Examples:
  render services update my-service --name my-new-name --output json
  render services update srv-abc123 --plan pro --output json
`,
}

func init() {
	servicesCmd.AddCommand(ServiceUpdateCmd)

	ServiceUpdateCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var cliInput servicetypes.ServiceUpdateInput
		if err := command.ParseCommand(cmd, args, &cliInput); err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}

		// Interactive mode is not implemented yet; force non-interactive behavior for now.
		command.DefaultFormatNonInteractive(cmd)

		cliInput = servicetypes.NormalizeServiceUpdateCLIInput(cliInput)
		if err := cliInput.ValidateUpdate(); err != nil {
			return err
		}

		ctx := cmd.Context()

		nonInteractive, err := command.NonInteractiveWithConfirm(cmd, func() (*client.Service, error) {
			return updateServiceNonInteractive(ctx, cliInput)
		}, func(svc *client.Service) string {
			return text.FormatStringF("Updated service %s (%s)", svc.Name, svc.Id)
		}, nil)
		if err != nil {
			return err
		}
		if nonInteractive {
			return nil
		}

		// TODO: Implement interactive TUI mode in a later phase
		return fmt.Errorf("interactive mode not yet implemented")
	}

	// Identity and source flags
	ServiceUpdateCmd.Flags().String("name", "", "Service name")
	ServiceUpdateCmd.Flags().String("repo", "", "Git repository URL")
	ServiceUpdateCmd.Flags().String("branch", "", "Git branch")
	ServiceUpdateCmd.Flags().String("image", "", "Docker image URL")

	// Deployment configuration flags
	ServiceUpdateCmd.Flags().String("plan", "", "Service plan")
	runtimeFlag := command.NewEnumInput(servicetypes.ServiceRuntimeValues(), false)
	ServiceUpdateCmd.Flags().Var(runtimeFlag, "runtime", "Runtime environment")
	ServiceUpdateCmd.Flags().String("root-directory", "", "Root directory")

	// Build and start commands
	ServiceUpdateCmd.Flags().String("build-command", "", "Build command")
	ServiceUpdateCmd.Flags().String("start-command", "", "Start command")
	ServiceUpdateCmd.Flags().String("pre-deploy-command", "", "Pre-deploy command")

	// Type-specific flags
	ServiceUpdateCmd.Flags().String("health-check-path", "", "Health check path")
	ServiceUpdateCmd.Flags().String("publish-directory", "", "Publish directory")
	ServiceUpdateCmd.Flags().String("cron-command", "", "Cron command")
	ServiceUpdateCmd.Flags().String("cron-schedule", "", "Cron schedule")

	// Registry flag
	ServiceUpdateCmd.Flags().String("registry-credential", "", "Registry credential")

	// Behavior flags
	ServiceUpdateCmd.Flags().Bool("auto-deploy", false, "Enable auto-deploy")

	// Build filter flags
	ServiceUpdateCmd.Flags().StringArray("build-filter-path", nil, "Build filter path (can be specified multiple times)")
	ServiceUpdateCmd.Flags().StringArray("build-filter-ignored-path", nil, "Build filter ignored path (can be specified multiple times)")

	// Instance and scaling flags
	ServiceUpdateCmd.Flags().Int("num-instances", 0, "Number of instances")
	ServiceUpdateCmd.Flags().Int("max-shutdown-delay", 0, "Max shutdown delay in seconds")

	// Preview and preview generation flags
	previewsFlag := command.NewEnumInput(servicetypes.PreviewsGenerationValues(), false)
	ServiceUpdateCmd.Flags().Var(previewsFlag, "previews", "Preview generation mode")

	// Maintenance mode flags
	ServiceUpdateCmd.Flags().Bool("maintenance-mode", false, "Enable maintenance mode")
	ServiceUpdateCmd.Flags().String("maintenance-mode-uri", "", "Maintenance mode URI")

	// IP allow list flag
	ServiceUpdateCmd.Flags().StringArray("ip-allow-list", nil, "IP allow list entry in cidr=...,description=... format (can be specified multiple times)")
}

func updateServiceNonInteractive(ctx context.Context, cliInput servicetypes.ServiceUpdateInput) (*client.Service, error) {
	deps := dependencies.GetFromContext(ctx)
	serviceRepo := deps.ServiceRepo()

	// Resolve service ID
	idOrName, err := cliInput.ParseServiceID()
	if err != nil {
		return nil, err
	}

	serviceID, err := serviceRepo.ResolveServiceIDFromNameOrID(ctx, idOrName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve service %q: %w", idOrName, err)
	}

	// TODO: BuildUpdateRequest to construct the PATCH body
	// For now, use an empty UpdateServiceJSONRequestBody as a placeholder
	body := client.UpdateServiceJSONRequestBody{}

	return serviceRepo.UpdateService(ctx, serviceID, body)
}

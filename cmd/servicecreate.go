package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/types"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
)

var ServiceCreateCmd = &cobra.Command{
	Use:   "create",
	Args:  cobra.NoArgs,
	Short: "Create a new service",
	Long: `Create a new service on Render.

This command currently runs in non-interactive mode only.
Provide all config with flags.

Examples:
  render services create --name my-api --type web_service --repo https://github.com/org/repo --output json
  render services create --from srv-abc123 --name my-api-clone --output json
`,
}

func init() {
	servicesCmd.AddCommand(ServiceCreateCmd)

	ServiceCreateCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var cliInput servicetypes.Service
		if err := command.ParseCommand(cmd, args, &cliInput); err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}

		// Interactive mode is not implemented yet; force non-interactive behavior for now.
		command.DefaultFormatNonInteractive(cmd)

		ctx := cmd.Context()

		nonInteractive, err := command.NonInteractive(cmd, func() (*client.Service, error) {
			return createServiceNonInteractive(ctx, cliInput)
		}, func(svc *client.Service) string {
			return text.FormatStringF("Created service %s (%s)", svc.Name, svc.Id)
		})
		if err != nil {
			return err
		}
		if nonInteractive {
			return nil
		}

		// TODO: Implement interactive TUI mode in a later phase
		// This will use views.NewServiceCreateView for guided configuration
		return fmt.Errorf("interactive mode not yet implemented")
	}

	ServiceCreateCmd.Flags().String("name", "", "Service name")
	serviceTypeFlag := command.NewEnumInput(servicetypes.ServiceTypeValues(), false)
	ServiceCreateCmd.Flags().Var(serviceTypeFlag, "type", "Service type")
	ServiceCreateCmd.Flags().String("from", "", "Clone configuration from existing service (ID or name). Other flags override cloned values.")
	ServiceCreateCmd.Flags().String("repo", "", "Git repository URL")
	ServiceCreateCmd.Flags().String("branch", "", "Git branch")
	ServiceCreateCmd.Flags().String("image", "", "Docker image URL")
	regionFlag := command.NewEnumInput(types.RegionValues(), false)
	ServiceCreateCmd.Flags().Var(regionFlag, "region", "Deployment region")
	ServiceCreateCmd.Flags().String("plan", "", "Service plan")
	runtimeFlag := command.NewEnumInput(servicetypes.ServiceRuntimeValues(), false)
	ServiceCreateCmd.Flags().Var(runtimeFlag, "runtime", "Runtime environment")
	ServiceCreateCmd.Flags().String("root-directory", "", "Root directory")
	ServiceCreateCmd.Flags().String("build-command", "", "Build command")
	ServiceCreateCmd.Flags().String("start-command", "", "Start command")
	ServiceCreateCmd.Flags().String("health-check-path", "", "Health check path")
	ServiceCreateCmd.Flags().String("publish-directory", "", "Publish directory")
	ServiceCreateCmd.Flags().String("cron-command", "", "Cron command")
	ServiceCreateCmd.Flags().String("cron-schedule", "", "Cron schedule")
	ServiceCreateCmd.Flags().String("environment-id", "", "Environment ID")
	ServiceCreateCmd.Flags().StringArray("env-var", nil, "Environment variable in KEY=VALUE format (can be specified multiple times)")
	ServiceCreateCmd.Flags().StringArray("secret-file", nil, "Secret file in NAME:LOCAL_PATH format (can be specified multiple times)")
	ServiceCreateCmd.Flags().String("registry-credential", "", "Registry credential")
	ServiceCreateCmd.Flags().Bool("auto-deploy", true, "Enable auto-deploy")
	ServiceCreateCmd.Flags().String("pre-deploy-command", "", "Pre-deploy command")
	ServiceCreateCmd.Flags().StringArray("build-filter-path", nil, "Build filter path (can be specified multiple times)")
	ServiceCreateCmd.Flags().StringArray("build-filter-ignored-path", nil, "Build filter ignored path (can be specified multiple times)")
	ServiceCreateCmd.Flags().Int("num-instances", 0, "Number of instances")
	ServiceCreateCmd.Flags().Int("max-shutdown-delay", 0, "Max shutdown delay in seconds")
	previewsFlag := command.NewEnumInput(servicetypes.PreviewsGenerationValues(), false)
	ServiceCreateCmd.Flags().Var(previewsFlag, "previews", "Preview generation mode")
	ServiceCreateCmd.Flags().Bool("maintenance-mode", false, "Enable maintenance mode")
	ServiceCreateCmd.Flags().String("maintenance-mode-uri", "", "Maintenance mode URI")
	ServiceCreateCmd.Flags().StringArray("ip-allow-list", nil, "IP allow list entry in cidr=...,description=... format (can be specified multiple times)")
}

func createServiceNonInteractive(ctx context.Context, cliInput servicetypes.Service) (*client.Service, error) {
	deps := dependencies.GetFromContext(ctx)
	serviceRepo := deps.ServiceRepo()
	registryService := deps.RegistryService()

	ownerID, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	cliInput = servicetypes.NormalizeServiceCreateCLIInput(cliInput)

	if cliInput.From != nil {
		if err := getConfigFromService(ctx, serviceRepo, &cliInput); err != nil {
			return nil, fmt.Errorf("failed to clone configuration from source service %q: %w", *cliInput.From, err)
		}
	}

	cliInput, err = servicetypes.NormalizeAndValidateCreateInput(cliInput, false)
	if err != nil {
		return nil, err
	}

	if cliInput.RegistryCredential != nil && cliInput.SupportsRegistryCredentials() {
		registryCredentialID, err := registryService.FindOneRegistryCredentialByIDFromNameOrID(ctx, ownerID, *cliInput.RegistryCredential)
		if err != nil {
			return nil, err
		}
		cliInput.RegistryCredential = &registryCredentialID
	}

	body, err := service.BuildCreateRequest(cliInput, ownerID)
	if err != nil {
		return nil, err
	}
	return serviceRepo.CreateService(ctx, body)
}

func getConfigFromService(ctx context.Context, repo *service.Repo, input *servicetypes.Service) error {
	serviceID, err := repo.ResolveServiceIDFromNameOrID(ctx, *input.From)
	if err != nil {
		return fmt.Errorf("failed to resolve source service %s: %w", *input.From, err)
	}
	input.From = &serviceID

	sourceService, err := repo.GetService(ctx, serviceID)
	if err != nil {
		return fmt.Errorf("failed to load source service %s: %w", serviceID, err)
	}
	service.ServiceFromAPI(input, sourceService)
	return nil
}

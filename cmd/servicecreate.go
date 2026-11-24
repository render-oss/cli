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

var ServiceCreateCmd = newServiceCreateCmd()

func init() {
	servicesCmd.AddCommand(ServiceCreateCmd)
}

func newServiceCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new service or clone an existing one",
		Long:  `Create a new service on Render. This command only runs in non-interactive modes. Provide configuration options with flags.`,
		Example: `  # Create a service from repository configuration
  render services create --name my-api --type web_service --repo https://github.com/org/repo --runtime node --build-command "npm install" --start-command "npm start" --output json

  # Clone configuration from an existing service
  render services create --from srv-abc123 --name my-api-clone --output json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var cliInput servicetypes.ServiceCreateInput
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

	cmd.Flags().String("name", "", "Set the service name")
	serviceTypeFlag := command.NewEnumInput(servicetypes.ServiceTypeValues(), false)
	cmd.Flags().Var(serviceTypeFlag, "type", "Set the service type")
	cmd.Flags().String("from", "", "Clone configuration from an existing service ID or name and override cloned values with other flags")
	cmd.Flags().String("repo", "", "Set the Git repository URL")
	cmd.Flags().String("branch", "", "Set the Git branch")
	cmd.Flags().String("image", "", "Set the Docker image URL")
	regionFlag := command.NewEnumInput(types.RegionValues(), false)
	cmd.Flags().Var(regionFlag, "region", "Set the deployment region")
	cmd.Flags().String("plan", "", "Set the service plan")
	runtimeFlag := command.NewEnumInput(servicetypes.ServiceRuntimeValues(), false)
	cmd.Flags().Var(runtimeFlag, "runtime", "Set the runtime environment")
	cmd.Flags().String("root-directory", "", "Set the root directory")
	cmd.Flags().String("build-command", "", "Set the build command")
	cmd.Flags().String("start-command", "", "Set the start command")
	cmd.Flags().String("health-check-path", "", "Set the health check path")
	cmd.Flags().String("publish-directory", "", "Set the publish directory")
	cmd.Flags().String("cron-command", "", "Set the cron command")
	cmd.Flags().String("cron-schedule", "", "Set the cron schedule")
	cmd.Flags().String("environment-id", "", "Set the environment ID")
	cmd.Flags().StringArray("env-var", nil, "Set environment variables in KEY=VALUE format (can be specified multiple times)")
	cmd.Flags().StringArray("secret-file", nil, "Set secret files in NAME:LOCAL_PATH format (can be specified multiple times)")
	cmd.Flags().String("registry-credential", "", "Set the registry credential")
	cmd.Flags().Bool("auto-deploy", true, "Enable auto-deploy")
	cmd.Flags().String("pre-deploy-command", "", "Set the pre-deploy command")
	cmd.Flags().StringArray("build-filter-path", nil, "Set build filter paths (can be specified multiple times)")
	cmd.Flags().StringArray("build-filter-ignored-path", nil, "Set build filter ignored paths (can be specified multiple times)")
	cmd.Flags().Int("num-instances", 0, "Set the number of instances")
	cmd.Flags().Int("max-shutdown-delay", 0, "Set max shutdown delay in seconds")
	previewsFlag := command.NewEnumInput(servicetypes.PreviewsGenerationValues(), false)
	cmd.Flags().Var(previewsFlag, "previews", "Set preview generation mode")
	cmd.Flags().Bool("maintenance-mode", false, "Enable maintenance mode")
	cmd.Flags().String("maintenance-mode-uri", "", "Set the maintenance mode URI")
	cmd.Flags().StringArray("ip-allow-list", nil, "Set IP allow list entries in cidr=..., description=... format (can be specified multiple times)")

	setFlagPlaceholder(cmd.Flags(), "name", "NAME")
	setFlagPlaceholder(cmd.Flags(), "type", "TYPE")
	setFlagPlaceholder(cmd.Flags(), "from", "SERVICE_ID")
	setFlagPlaceholder(cmd.Flags(), "repo", "REPO_URL")
	setFlagPlaceholder(cmd.Flags(), "branch", "BRANCH")
	setFlagPlaceholder(cmd.Flags(), "image", "IMAGE_URL")
	setFlagPlaceholder(cmd.Flags(), "region", "REGION")
	setFlagPlaceholder(cmd.Flags(), "plan", "PLAN")
	setFlagPlaceholder(cmd.Flags(), "runtime", "RUNTIME")
	setFlagPlaceholder(cmd.Flags(), "root-directory", "PATH")
	setFlagPlaceholder(cmd.Flags(), "build-command", "COMMAND")
	setFlagPlaceholder(cmd.Flags(), "start-command", "COMMAND")
	setFlagPlaceholder(cmd.Flags(), "health-check-path", "PATH")
	setFlagPlaceholder(cmd.Flags(), "publish-directory", "PATH")
	setFlagPlaceholder(cmd.Flags(), "cron-command", "COMMAND")
	setFlagPlaceholder(cmd.Flags(), "cron-schedule", "SCHEDULE")
	setFlagPlaceholder(cmd.Flags(), "environment-id", "ENVIRONMENT_ID")
	setFlagPlaceholder(cmd.Flags(), "env-var", "KEY_VALUE")
	setFlagPlaceholder(cmd.Flags(), "secret-file", "NAME_PATH")
	setFlagPlaceholder(cmd.Flags(), "registry-credential", "CREDENTIAL")
	setFlagPlaceholder(cmd.Flags(), "pre-deploy-command", "COMMAND")
	setFlagPlaceholder(cmd.Flags(), "build-filter-path", "PATHS")
	setFlagPlaceholder(cmd.Flags(), "build-filter-ignored-path", "PATHS")
	setFlagPlaceholder(cmd.Flags(), "num-instances", "COUNT")
	setFlagPlaceholder(cmd.Flags(), "max-shutdown-delay", "SECONDS")
	setFlagPlaceholder(cmd.Flags(), "previews", "PREVIEWS")
	setFlagPlaceholder(cmd.Flags(), "maintenance-mode-uri", "URI")
	setFlagPlaceholder(cmd.Flags(), "ip-allow-list", "CIDR_DESCRIPTION")

	return cmd
}

func createServiceNonInteractive(ctx context.Context, cliInput servicetypes.ServiceCreateInput) (*client.Service, error) {
	deps := dependencies.GetFromContext(ctx)
	serviceRepo := deps.ServiceRepo()
	registryService := deps.RegistryService()

	ownerID, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	rawInput := cliInput
	cliInput = servicetypes.NormalizeServiceCreateCLIInput(cliInput)

	if cliInput.From != nil {
		// A blank clearable flag replaces the source's value instead of inheriting it.
		cliInput = servicetypes.NormalizeServiceCloneCLIInput(rawInput)
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

func getConfigFromService(ctx context.Context, repo *service.Repo, input *servicetypes.ServiceCreateInput) error {
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

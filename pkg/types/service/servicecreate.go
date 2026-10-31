package service

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	types "github.com/render-oss/cli/pkg/types"
)

// ServiceCreateInput is the raw command input parsed from Cobra flags for service creation.
type ServiceCreateInput struct {
	Name string       `cli:"name"`
	Type *ServiceType `cli:"type"`
	From *string      `cli:"from"`

	Repo   *string `cli:"repo"`
	Branch *string `cli:"branch"`
	Image  *string `cli:"image"`

	Region        *types.Region   `cli:"region"`
	Plan          *string         `cli:"plan"`
	Runtime       *ServiceRuntime `cli:"runtime"`
	RootDirectory *string         `cli:"root-directory"`

	BuildCommand *string `cli:"build-command"`
	StartCommand *string `cli:"start-command"`

	HealthCheckPath  *string `cli:"health-check-path"`
	PublishDirectory *string `cli:"publish-directory"`

	CronCommand  *string `cli:"cron-command"`
	CronSchedule *string `cli:"cron-schedule"`

	EnvironmentID *string  `cli:"environment-id"`
	EnvVars       []string `cli:"env-var"`
	SecretFiles   []string `cli:"secret-file"`

	RegistryCredential *string `cli:"registry-credential"`

	AutoDeploy       *bool   `cli:"auto-deploy"`
	PreDeployCommand *string `cli:"pre-deploy-command"`

	BuildFilterPaths        []string            `cli:"build-filter-path"`
	BuildFilterIgnoredPaths []string            `cli:"build-filter-ignored-path"`
	NumInstances            *int                `cli:"num-instances"`
	MaxShutdownDelay        *int                `cli:"max-shutdown-delay"`
	Previews                *PreviewsGeneration `cli:"previews"`
	MaintenanceMode         *bool               `cli:"maintenance-mode"`
	MaintenanceModeURI      *string             `cli:"maintenance-mode-uri"`
	IPAllowList             []string            `cli:"ip-allow-list"`
}

func (s ServiceCreateInput) OptionalServiceType() (*ServiceType, error) {
	return OptionalServiceType(s.Type)
}

func (s ServiceCreateInput) OptionalServiceRuntime() (*ServiceRuntime, error) {
	return OptionalServiceRuntime(s.Runtime)
}

func (s ServiceCreateInput) OptionalRegion() (*types.Region, error) {
	return types.OptionalRegion(s.Region)
}

// NormalizeAndValidateCreateInput normalizes and validates CLI input for service creation.
func NormalizeAndValidateCreateInput(input ServiceCreateInput, isInteractive bool) (ServiceCreateInput, error) {
	normalized := NormalizeServiceCreateCLIInput(input)
	if err := normalized.validateNormalized(isInteractive); err != nil {
		return ServiceCreateInput{}, err
	}
	return normalized, nil
}

func (s ServiceCreateInput) Validate(isInteractive bool) error {
	normalized := NormalizeServiceCreateCLIInput(s)
	return normalized.validateNormalized(isInteractive)
}

func (s ServiceCreateInput) validateNormalized(isInteractive bool) error {
	if isInteractive {
		return nil
	}

	if s.Name == "" {
		return errors.New("name is required")
	}
	if s.From == nil && s.Type == nil {
		return errors.New("type is required")
	}

	if s.Repo != nil && s.Image != nil {
		return errors.New("cannot specify both --repo and --image")
	}

	if s.Repo == nil && s.Image == nil && s.From == nil {
		return errors.New("either repo or image is required")
	}

	parsedRuntime, err := s.OptionalServiceRuntime()
	if err != nil {
		return err
	}

	parsedType, err := s.OptionalServiceType()
	if err != nil {
		return err
	}

	if _, err := s.OptionalRegion(); err != nil {
		return err
	}

	if usesNativeCommandFlags(s) && resolvesToImageRuntime(s) {
		return errors.New("--build-command and --start-command are only supported for native runtimes")
	}

	if s.RegistryCredential != nil && !s.SupportsRegistryCredentials() {
		return errors.New("--registry-credential is only supported with --image or --runtime docker/image")
	}

	if s.Previews != nil {
		if _, err := ParsePreviewsGeneration(string(*s.Previews)); err != nil {
			return err
		}
	}

	for _, entry := range s.IPAllowList {
		if _, _, err := types.ParseIPAllowListEntry(entry); err != nil {
			return err
		}
	}

	if s.MaintenanceModeURI != nil && s.MaintenanceMode == nil {
		return errors.New("cannot set --maintenance-mode-uri without --maintenance-mode")
	}

	if parsedType != nil {
		if err := validateFlagsForType(*parsedType, s); err != nil {
			return err
		}
	}

	if parsedType != nil && *parsedType == ServiceTypeCronJob {
		if s.From == nil && (s.CronCommand == nil || s.CronSchedule == nil) {
			return errors.New("cron-command and cron-schedule are required for cron jobs")
		}
	}

	if parsedRuntime == nil && s.Image == nil && s.From == nil && serviceTypeRequiresRuntime(parsedType) {
		return errors.New("--runtime is required when not providing --image")
	}

	if parsedRuntime != nil && parsedRuntime.IsNative() {
		if parsedType != nil && *parsedType == ServiceTypeCronJob {
			if s.BuildCommand == nil {
				return errors.New("--build-command is required for cron jobs when runtime is native")
			}
		} else if parsedType != nil && *parsedType != ServiceTypeStaticSite {
			if s.BuildCommand == nil || s.StartCommand == nil {
				return errors.New("--build-command and --start-command are required when runtime is native")
			}
		}
	}

	if err := validateEnvVarFlags(s.EnvVars); err != nil {
		return err
	}
	if err := validateSecretFileFlags(s.SecretFiles); err != nil {
		return err
	}

	return nil
}

func (s ServiceCreateInput) SupportsRegistryCredentials() bool {
	if s.Image != nil {
		return true
	}

	parsedRuntime, err := s.OptionalServiceRuntime()
	if err != nil || parsedRuntime == nil {
		return false
	}

	if *parsedRuntime == ServiceRuntimeDocker {
		return true
	}

	if *parsedRuntime == ServiceRuntimeImage {
		return true
	}

	return false
}

func usesNativeCommandFlags(s ServiceCreateInput) bool {
	return s.BuildCommand != nil || s.StartCommand != nil
}

func resolvesToImageRuntime(s ServiceCreateInput) bool {
	if parsedRuntime, err := s.OptionalServiceRuntime(); err == nil && parsedRuntime != nil {
		return *parsedRuntime == ServiceRuntimeImage
	}

	return s.Image != nil
}

func serviceTypeRequiresRuntime(serviceType *ServiceType) bool {
	return serviceType == nil || *serviceType != ServiceTypeStaticSite
}

func NormalizeServiceCreateCLIInput(input ServiceCreateInput) ServiceCreateInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Type = types.OptionalAlias(input.Type)
	input.From = types.OptionalNonZeroString(input.From)
	input.Repo = types.OptionalNonZeroString(input.Repo)
	input.Branch = types.OptionalNonZeroString(input.Branch)
	input.Image = types.OptionalNonZeroString(input.Image)
	input.Region = types.OptionalAlias(input.Region)
	input.Plan = types.OptionalNonZeroString(input.Plan)
	input.Runtime = types.OptionalAlias(input.Runtime)
	input.RootDirectory = types.OptionalNonZeroString(input.RootDirectory)
	input.BuildCommand = types.OptionalNonZeroCommandText(input.BuildCommand)
	input.StartCommand = types.OptionalNonZeroCommandText(input.StartCommand)
	input.HealthCheckPath = types.OptionalNonZeroString(input.HealthCheckPath)
	input.PublishDirectory = types.OptionalNonZeroString(input.PublishDirectory)
	input.CronCommand = types.OptionalNonZeroCommandText(input.CronCommand)
	input.CronSchedule = types.OptionalNonZeroString(input.CronSchedule)
	input.EnvironmentID = types.OptionalNonZeroString(input.EnvironmentID)
	input.RegistryCredential = types.OptionalNonZeroString(input.RegistryCredential)
	input.PreDeployCommand = types.OptionalNonZeroCommandText(input.PreDeployCommand)
	input.Previews = types.OptionalAlias(input.Previews)
	input.MaintenanceModeURI = types.OptionalNonZeroString(input.MaintenanceModeURI)
	return input
}

// NormalizeServiceCloneCLIInput normalizes input for `services create --from`
// like NormalizeServiceCreateCLIInput, except that an optional setting the
// caller explicitly set to blank stays "" rather than becoming nil, so the
// clone clears it instead of inheriting the source's value.
// NormalizeAndValidateCreateInput then omits it from the request, as it does
// for a direct create.
//
// Only settings whose blank value means "none" are kept: the root directory
// (the repository root), the health check path and the pre-deploy command. Any
// other blank flag still counts as not given and inherits the source's value.
func NormalizeServiceCloneCLIInput(input ServiceCreateInput) ServiceCreateInput {
	normalized := NormalizeServiceCreateCLIInput(input)
	normalized.RootDirectory = keepExplicitBlank(input.RootDirectory, normalized.RootDirectory)
	normalized.HealthCheckPath = keepExplicitBlank(input.HealthCheckPath, normalized.HealthCheckPath)
	normalized.PreDeployCommand = keepExplicitBlank(input.PreDeployCommand, normalized.PreDeployCommand)
	return normalized
}

// keepExplicitBlank returns "" for a value that was set but normalized away as blank.
func keepExplicitBlank(raw, normalized *string) *string {
	if raw != nil && normalized == nil {
		return new(string)
	}
	return normalized
}

func validateEnvVarFlags(envVars []string) error {
	for _, envVar := range envVars {
		if _, err := types.ParseEnvVar(envVar); err != nil {
			return err
		}
	}

	return nil
}

func validateSecretFileFlags(secretFiles []string) error {
	for _, secretFile := range secretFiles {
		if _, err := ParseSecretFileRef(secretFile); err != nil {
			return err
		}
	}

	return nil
}

func validateFlagsForType(serviceType ServiceType, s ServiceCreateInput) error {
	reject := func(flag string, allowedTypes ...ServiceType) error {
		if slices.Contains(allowedTypes, serviceType) {
			return nil
		}
		return fmt.Errorf("--%s is not supported for %s", flag, serviceType)
	}

	if s.CronCommand != nil {
		if err := reject("cron-command", ServiceTypeCronJob); err != nil {
			return err
		}
	}
	if s.CronSchedule != nil {
		if err := reject("cron-schedule", ServiceTypeCronJob); err != nil {
			return err
		}
	}
	if s.HealthCheckPath != nil {
		if err := reject("health-check-path", ServiceTypeWebService); err != nil {
			return err
		}
	}
	if s.PublishDirectory != nil {
		if err := reject("publish-directory", ServiceTypeStaticSite); err != nil {
			return err
		}
	}
	if s.StartCommand != nil {
		if err := reject("start-command", ServiceTypeWebService, ServiceTypePrivateService, ServiceTypeBackgroundWorker); err != nil {
			return err
		}
	}
	if s.PreDeployCommand != nil {
		if err := reject("pre-deploy-command", ServiceTypeWebService, ServiceTypePrivateService, ServiceTypeBackgroundWorker); err != nil {
			return err
		}
	}
	if s.NumInstances != nil {
		if err := reject("num-instances", ServiceTypeWebService, ServiceTypePrivateService, ServiceTypeBackgroundWorker); err != nil {
			return err
		}
	}
	if s.MaxShutdownDelay != nil {
		if err := reject("max-shutdown-delay", ServiceTypeWebService, ServiceTypePrivateService, ServiceTypeBackgroundWorker); err != nil {
			return err
		}
	}
	if s.Previews != nil {
		if err := reject("previews", ServiceTypeWebService, ServiceTypePrivateService, ServiceTypeBackgroundWorker, ServiceTypeStaticSite); err != nil {
			return err
		}
	}
	if s.MaintenanceMode != nil {
		if err := reject("maintenance-mode", ServiceTypeWebService); err != nil {
			return err
		}
	}
	if s.MaintenanceModeURI != nil {
		if err := reject("maintenance-mode-uri", ServiceTypeWebService); err != nil {
			return err
		}
	}
	if len(s.IPAllowList) > 0 {
		if err := reject("ip-allow-list", ServiceTypeWebService, ServiceTypeStaticSite); err != nil {
			return err
		}
	}

	parsedRuntime, _ := s.OptionalServiceRuntime()
	if parsedRuntime != nil {
		if err := reject("runtime", ServiceTypeWebService, ServiceTypePrivateService, ServiceTypeBackgroundWorker, ServiceTypeCronJob); err != nil {
			return err
		}
	}
	if s.Region != nil {
		if err := reject("region", ServiceTypeWebService, ServiceTypePrivateService, ServiceTypeBackgroundWorker, ServiceTypeCronJob); err != nil {
			return err
		}
	}
	if s.Plan != nil {
		if err := reject("plan", ServiceTypeWebService, ServiceTypePrivateService, ServiceTypeBackgroundWorker, ServiceTypeCronJob); err != nil {
			return err
		}
	}

	return nil
}

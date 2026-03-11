package service

import (
	"errors"
	"strings"

	types "github.com/render-oss/cli/pkg/types"
)

// Service is the raw command input parsed from Cobra flags.
type Service struct {
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
}

func (s Service) OptionalServiceType() (*ServiceType, error) {
	return OptionalServiceType(s.Type)
}

func (s Service) OptionalServiceRuntime() (*ServiceRuntime, error) {
	return OptionalServiceRuntime(s.Runtime)
}

func (s Service) OptionalRegion() (*types.Region, error) {
	return types.OptionalRegion(s.Region)
}

// NormalizeAndValidateCreateInput normalizes and validates CLI input for service creation.
func NormalizeAndValidateCreateInput(input Service, isInteractive bool) (Service, error) {
	normalized := NormalizeServiceCreateCLIInput(input)
	if err := normalized.validateNormalized(isInteractive); err != nil {
		return Service{}, err
	}
	return normalized, nil
}

func (s Service) Validate(isInteractive bool) error {
	normalized := NormalizeServiceCreateCLIInput(s)
	return normalized.validateNormalized(isInteractive)
}

func NormalizeServiceCreateCLIInput(input Service) Service {
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
	input.BuildCommand = types.OptionalNonZeroString(input.BuildCommand)
	input.StartCommand = types.OptionalNonZeroString(input.StartCommand)
	input.HealthCheckPath = types.OptionalNonZeroString(input.HealthCheckPath)
	input.PublishDirectory = types.OptionalNonZeroString(input.PublishDirectory)
	input.CronCommand = types.OptionalNonZeroString(input.CronCommand)
	input.CronSchedule = types.OptionalNonZeroString(input.CronSchedule)
	input.EnvironmentID = types.OptionalNonZeroString(input.EnvironmentID)
	input.RegistryCredential = types.OptionalNonZeroString(input.RegistryCredential)
	input.PreDeployCommand = types.OptionalNonZeroString(input.PreDeployCommand)
	return input
}

func (s Service) validateNormalized(isInteractive bool) error {
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

func (s Service) SupportsRegistryCredentials() bool {
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

func usesNativeCommandFlags(s Service) bool {
	return s.BuildCommand != nil || s.StartCommand != nil
}

func resolvesToImageRuntime(s Service) bool {
	if parsedRuntime, err := s.OptionalServiceRuntime(); err == nil && parsedRuntime != nil {
		return *parsedRuntime == ServiceRuntimeImage
	}

	return s.Image != nil
}

func serviceTypeRequiresRuntime(serviceType *ServiceType) bool {
	return serviceType == nil || *serviceType != ServiceTypeStaticSite
}

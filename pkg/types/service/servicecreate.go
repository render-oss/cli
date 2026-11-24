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
	return slices.ContainsFunc(s.restrictedFields(), func(f restrictedField) bool { return f.set && f.nativeOnly })
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
	input.BuildCommand = types.OptionalNonZeroString(input.BuildCommand)
	input.StartCommand = types.OptionalNonZeroString(input.StartCommand)
	input.HealthCheckPath = types.OptionalNonZeroString(input.HealthCheckPath)
	input.PublishDirectory = types.OptionalNonZeroString(input.PublishDirectory)
	input.CronCommand = types.OptionalNonZeroString(input.CronCommand)
	input.CronSchedule = types.OptionalNonZeroString(input.CronSchedule)
	input.EnvironmentID = types.OptionalNonZeroString(input.EnvironmentID)
	input.RegistryCredential = types.OptionalNonZeroString(input.RegistryCredential)
	input.PreDeployCommand = types.OptionalNonZeroString(input.PreDeployCommand)
	input.Previews = types.OptionalAlias(input.Previews)
	input.MaintenanceModeURI = types.OptionalNonZeroString(input.MaintenanceModeURI)
	return input
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

// Service types that accept each type-specific flag.
var (
	cronJobTypes        = []ServiceType{ServiceTypeCronJob}
	webServiceTypes     = []ServiceType{ServiceTypeWebService}
	staticSiteTypes     = []ServiceType{ServiceTypeStaticSite}
	longRunningTypes    = []ServiceType{ServiceTypeWebService, ServiceTypePrivateService, ServiceTypeBackgroundWorker}
	previewTypes        = []ServiceType{ServiceTypeWebService, ServiceTypePrivateService, ServiceTypeBackgroundWorker, ServiceTypeStaticSite}
	ipAllowListTypes    = []ServiceType{ServiceTypeWebService, ServiceTypeStaticSite}
	computeServiceTypes = []ServiceType{ServiceTypeWebService, ServiceTypePrivateService, ServiceTypeBackgroundWorker, ServiceTypeCronJob}
)

// restrictedField is a flag that only some service types or runtimes accept.
type restrictedField struct {
	flag string
	// types lists the service types that accept the flag; nil means all of them.
	types []ServiceType
	// nativeOnly marks a flag that a service on the image runtime refuses.
	nativeOnly bool
	set        bool
	clear      func(*ServiceCreateInput)
}

func (f restrictedField) supportedBy(serviceType ServiceType) bool {
	return f.types == nil || slices.Contains(f.types, serviceType)
}

// restrictedFields lists the flags that only some service types or runtimes
// accept, in the order validateFlagsForType checks them. Validation refuses
// such a flag when it is set; WithoutFieldsUnsupportedBy clears it instead.
func (s ServiceCreateInput) restrictedFields() []restrictedField {
	parsedRuntime, _ := s.OptionalServiceRuntime()
	return []restrictedField{
		{flag: "cron-command", types: cronJobTypes, set: s.CronCommand != nil, clear: func(s *ServiceCreateInput) { s.CronCommand = nil }},
		{flag: "cron-schedule", types: cronJobTypes, set: s.CronSchedule != nil, clear: func(s *ServiceCreateInput) { s.CronSchedule = nil }},
		{flag: "health-check-path", types: webServiceTypes, set: s.HealthCheckPath != nil, clear: func(s *ServiceCreateInput) { s.HealthCheckPath = nil }},
		{flag: "publish-directory", types: staticSiteTypes, set: s.PublishDirectory != nil, clear: func(s *ServiceCreateInput) { s.PublishDirectory = nil }},
		{flag: "build-command", nativeOnly: true, set: s.BuildCommand != nil, clear: func(s *ServiceCreateInput) { s.BuildCommand = nil }},
		{flag: "start-command", types: longRunningTypes, nativeOnly: true, set: s.StartCommand != nil, clear: func(s *ServiceCreateInput) { s.StartCommand = nil }},
		{flag: "pre-deploy-command", types: longRunningTypes, set: s.PreDeployCommand != nil, clear: func(s *ServiceCreateInput) { s.PreDeployCommand = nil }},
		{flag: "num-instances", types: longRunningTypes, set: s.NumInstances != nil, clear: func(s *ServiceCreateInput) { s.NumInstances = nil }},
		{flag: "max-shutdown-delay", types: longRunningTypes, set: s.MaxShutdownDelay != nil, clear: func(s *ServiceCreateInput) { s.MaxShutdownDelay = nil }},
		{flag: "previews", types: previewTypes, set: s.Previews != nil, clear: func(s *ServiceCreateInput) { s.Previews = nil }},
		{flag: "maintenance-mode", types: webServiceTypes, set: s.MaintenanceMode != nil, clear: func(s *ServiceCreateInput) { s.MaintenanceMode = nil }},
		{flag: "maintenance-mode-uri", types: webServiceTypes, set: s.MaintenanceModeURI != nil, clear: func(s *ServiceCreateInput) { s.MaintenanceModeURI = nil }},
		{flag: "ip-allow-list", types: ipAllowListTypes, set: len(s.IPAllowList) > 0, clear: func(s *ServiceCreateInput) { s.IPAllowList = nil }},
		{flag: "runtime", types: computeServiceTypes, set: parsedRuntime != nil, clear: func(s *ServiceCreateInput) { s.Runtime = nil }},
		{flag: "region", types: computeServiceTypes, set: s.Region != nil, clear: func(s *ServiceCreateInput) { s.Region = nil }},
		{flag: "plan", types: computeServiceTypes, set: s.Plan != nil, clear: func(s *ServiceCreateInput) { s.Plan = nil }},
	}
}

func validateFlagsForType(serviceType ServiceType, s ServiceCreateInput) error {
	for _, f := range s.restrictedFields() {
		if f.set && !f.supportedBy(serviceType) {
			return fmt.Errorf("--%s is not supported for %s", f.flag, serviceType)
		}
	}
	return nil
}

// WithoutFieldsUnsupportedBy returns s with every field cleared that
// destination would refuse, given its service type and runtime. Cloning
// applies it to the source service's settings, so a clone of another type or
// runtime inherits only the settings it accepts; flags the caller sets are not
// filtered and still fail validation.
func (s ServiceCreateInput) WithoutFieldsUnsupportedBy(destination ServiceCreateInput) ServiceCreateInput {
	// Validation refuses an unknown type, which leaves nothing to filter by.
	serviceType, _ := destination.OptionalServiceType()
	imageRuntime := resolvesToImageRuntime(destination)
	for _, f := range s.restrictedFields() {
		if (serviceType != nil && !f.supportedBy(*serviceType)) || (imageRuntime && f.nativeOnly) {
			f.clear(&s)
		}
	}
	return s
}

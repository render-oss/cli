package service

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	types "github.com/render-oss/cli/pkg/types"
)

// ServiceUpdateInput is the raw command input parsed from Cobra flags for service update.
type ServiceUpdateInput struct {
	Name string `cli:"name"`

	Repo   *string `cli:"repo"`
	Branch *string `cli:"branch"`
	Image  *string `cli:"image"`

	Plan          *string         `cli:"plan"`
	Runtime       *ServiceRuntime `cli:"runtime"`
	RootDirectory *string         `cli:"root-directory"`

	BuildCommand *string `cli:"build-command"`
	StartCommand *string `cli:"start-command"`

	HealthCheckPath  *string `cli:"health-check-path"`
	PublishDirectory *string `cli:"publish-directory"`

	CronCommand  *string `cli:"cron-command"`
	CronSchedule *string `cli:"cron-schedule"`

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

	ServiceIDOrName string `cli:"arg:0"`
}

// ParseServiceID returns the service identifier from the positional arg.
// It errors if no service identifier is provided.
func (s ServiceUpdateInput) ParseServiceID() (string, error) {
	trimmed := strings.TrimSpace(s.ServiceIDOrName)
	if trimmed == "" {
		return "", errors.New("service ID or name is required")
	}
	return trimmed, nil
}

func NormalizeServiceUpdateCLIInput(input ServiceUpdateInput) ServiceUpdateInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Repo = types.TrimOptionalString(input.Repo)
	input.Branch = types.TrimOptionalString(input.Branch)
	input.Image = types.TrimOptionalString(input.Image)
	input.Plan = types.TrimOptionalString(input.Plan)
	input.Runtime = types.OptionalAlias(input.Runtime)
	input.RootDirectory = types.TrimOptionalString(input.RootDirectory)
	input.BuildCommand = types.TrimOptionalString(input.BuildCommand)
	input.StartCommand = types.TrimOptionalString(input.StartCommand)
	input.HealthCheckPath = types.TrimOptionalString(input.HealthCheckPath)
	input.PublishDirectory = types.TrimOptionalString(input.PublishDirectory)
	input.CronCommand = types.TrimOptionalString(input.CronCommand)
	input.CronSchedule = types.TrimOptionalString(input.CronSchedule)
	input.RegistryCredential = types.TrimOptionalString(input.RegistryCredential)
	input.PreDeployCommand = types.TrimOptionalString(input.PreDeployCommand)
	input.Previews = types.OptionalAlias(input.Previews)
	input.MaintenanceModeURI = types.TrimOptionalString(input.MaintenanceModeURI)
	return input
}

// NormalizeAndValidateUpdateInput normalizes and validates CLI input for service update.
func NormalizeAndValidateUpdateInput(input ServiceUpdateInput) (ServiceUpdateInput, error) {
	normalized := NormalizeServiceUpdateCLIInput(input)
	if err := normalized.ValidateUpdate(); err != nil {
		return ServiceUpdateInput{}, err
	}
	return normalized, nil
}

// ValidateUpdate checks that a service identifier is provided and at least one update flag is set.
// It does not validate flag/type compatibility — call ValidateForServiceType after
// resolving the service type from the API.
func (s ServiceUpdateInput) ValidateUpdate() error {
	if _, err := s.ParseServiceID(); err != nil {
		return err
	}
	if !s.hasUpdateFlags() {
		return errors.New("at least one update flag must be provided")
	}
	if s.Previews != nil {
		if _, err := ParsePreviewsGeneration(string(*s.Previews)); err != nil {
			return err
		}
	}
	for _, entry := range s.IPAllowList {
		if _, _, err := ParseIPAllowListEntry(entry); err != nil {
			return err
		}
	}
	if s.MaintenanceModeURI != nil && s.MaintenanceMode == nil {
		return errors.New("cannot set --maintenance-mode-uri without --maintenance-mode")
	}
	return nil
}

// hasUpdateFlags returns true if any field is set.
// This is used to check that at least one update flag is provided, since all fields are optional.
func (s ServiceUpdateInput) hasUpdateFlags() bool {
	// Identity and source
	if s.Name != "" || s.Repo != nil || s.Branch != nil || s.Image != nil {
		return true
	}

	// Build and deploy configuration
	if s.BuildCommand != nil || s.StartCommand != nil || s.PreDeployCommand != nil || s.AutoDeploy != nil {
		return true
	}

	// Runtime configuration
	if s.Runtime != nil || s.Plan != nil || s.RootDirectory != nil {
		return true
	}

	// Type-specific fields
	if s.HealthCheckPath != nil || s.PublishDirectory != nil {
		return true
	}
	if s.CronCommand != nil || s.CronSchedule != nil {
		return true
	}

	// Build filter and scaling fields
	if len(s.BuildFilterPaths) > 0 || len(s.BuildFilterIgnoredPaths) > 0 {
		return true
	}
	if s.NumInstances != nil || s.MaxShutdownDelay != nil || s.Previews != nil {
		return true
	}
	if s.MaintenanceMode != nil || s.MaintenanceModeURI != nil {
		return true
	}
	if len(s.IPAllowList) > 0 {
		return true
	}

	// Registry
	if s.RegistryCredential != nil {
		return true
	}

	return false
}

// ValidateForServiceType rejects flags that don't apply to the given service type.
// Call this after resolving the service type from the API.
func (s ServiceUpdateInput) ValidateForServiceType(serviceType ServiceType) error {
	reject := func(flag string, allowedTypes ...ServiceType) error {
		if slices.Contains(allowedTypes, serviceType) {
			return nil
		}
		return fmt.Errorf("--%s is not supported for %s services", flag, serviceType)
	}

	if s.NumInstances != nil {
		return fmt.Errorf("--num-instances is not supported for update (use the dashboard to change instance count)")
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
	if s.Plan != nil {
		if err := reject("plan", ServiceTypeWebService, ServiceTypePrivateService, ServiceTypeBackgroundWorker, ServiceTypeCronJob); err != nil {
			return err
		}
	}
	if s.Runtime != nil {
		if err := reject("runtime", ServiceTypeWebService, ServiceTypePrivateService, ServiceTypeBackgroundWorker, ServiceTypeCronJob); err != nil {
			return err
		}
	}

	return nil
}

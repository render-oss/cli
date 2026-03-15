package service_test

import (
	"testing"

	"github.com/render-oss/cli/pkg/pointers"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvedServiceID(t *testing.T) {
	t.Run("returns positional arg when set", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			ServiceIDOrName: "my-service-id",
		}
		id, err := svc.ParseServiceID()
		require.NoError(t, err)
		assert.Equal(t, "my-service-id", id)
	})

	t.Run("returns error when not set", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{}
		_, err := svc.ParseServiceID()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "service ID or name is required")
	})

	t.Run("trims whitespace", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			ServiceIDOrName: "  my-service  ",
		}
		id, err := svc.ParseServiceID()
		require.NoError(t, err)
		assert.Equal(t, "my-service", id)
	})

	t.Run("whitespace-only treated as missing", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			ServiceIDOrName: "   ",
		}
		_, err := svc.ParseServiceID()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "service ID or name is required")
	})
}

func TestValidateUpdate(t *testing.T) {
	t.Run("returns error when no service identifier", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			Name: "new-name",
		}
		err := svc.ValidateUpdate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "service ID or name is required")
	})

	t.Run("returns error when service ID set but no update flags", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			ServiceIDOrName: "my-service-id",
		}
		err := svc.ValidateUpdate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one update flag must be provided")
	})

	t.Run("passes with service ID and name flag", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			Name:            "new-name",
			ServiceIDOrName: "my-service-id",
		}
		err := svc.ValidateUpdate()
		require.NoError(t, err)
	})

	t.Run("passes with service ID and repo flag", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			Repo:            pointers.From("https://github.com/org/repo"),
			ServiceIDOrName: "my-service-id",
		}
		err := svc.ValidateUpdate()
		require.NoError(t, err)
	})

	t.Run("passes with service ID and build-command flag", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			BuildCommand:    pointers.From("npm ci"),
			ServiceIDOrName: "my-service-id",
		}
		err := svc.ValidateUpdate()
		require.NoError(t, err)
	})

	t.Run("passes with service ID and auto-deploy flag", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			AutoDeploy:      pointers.From(true),
			ServiceIDOrName: "my-service-id",
		}
		err := svc.ValidateUpdate()
		require.NoError(t, err)
	})

	t.Run("passes with service ID and runtime flag", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			Runtime:         svcRuntime(servicetypes.ServiceRuntimeNode),
			ServiceIDOrName: "my-service-id",
		}
		err := svc.ValidateUpdate()
		require.NoError(t, err)
	})

	t.Run("passes with service ID and num-instances flag", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			NumInstances:    pointers.From(3),
			ServiceIDOrName: "my-service-id",
		}
		err := svc.ValidateUpdate()
		require.NoError(t, err)
	})

	t.Run("passes with service ID and maintenance-mode flag", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			MaintenanceMode: pointers.From(true),
			ServiceIDOrName: "my-service-id",
		}
		err := svc.ValidateUpdate()
		require.NoError(t, err)
	})

	t.Run("passes with service ID and ip-allow-list flag", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			IPAllowList:     []string{"cidr=10.0.0.0/8"},
			ServiceIDOrName: "my-service-id",
		}
		err := svc.ValidateUpdate()
		require.NoError(t, err)
	})

	t.Run("passes with service ID and build-filter-path flag", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			BuildFilterPaths: []string{"src/**"},
			ServiceIDOrName:  "my-service-id",
		}
		err := svc.ValidateUpdate()
		require.NoError(t, err)
	})

	t.Run("rejects invalid previews value", func(t *testing.T) {
		previews := servicetypes.PreviewsGeneration("invalid")
		svc := servicetypes.ServiceUpdateInput{
			Previews:        &previews,
			ServiceIDOrName: "my-service-id",
		}
		err := svc.ValidateUpdate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "previews must be one of")
	})

	t.Run("accepts valid previews value", func(t *testing.T) {
		previews := servicetypes.PreviewsGeneration("automatic")
		svc := servicetypes.ServiceUpdateInput{
			Previews:        &previews,
			ServiceIDOrName: "my-service-id",
		}
		err := svc.ValidateUpdate()
		require.NoError(t, err)
	})

	t.Run("rejects invalid ip-allow-list entry", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			IPAllowList:     []string{"bad-entry"},
			ServiceIDOrName: "my-service-id",
		}
		err := svc.ValidateUpdate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --ip-allow-list")
	})

	t.Run("rejects --maintenance-mode-uri without --maintenance-mode", func(t *testing.T) {
		svc := servicetypes.ServiceUpdateInput{
			MaintenanceModeURI: pointers.From("/maintenance.html"),
			ServiceIDOrName:    "my-service-id",
		}
		err := svc.ValidateUpdate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot set --maintenance-mode-uri without --maintenance-mode")
	})
}

func TestNormalizeAndValidateUpdateInput(t *testing.T) {
	t.Run("normalizes and validates", func(t *testing.T) {
		input := servicetypes.ServiceUpdateInput{
			Name:            " new-name ",
			BuildCommand:    pointers.From("  npm ci  "),
			ServiceIDOrName: "my-service-id",
		}
		got, err := servicetypes.NormalizeAndValidateUpdateInput(input)
		require.NoError(t, err)
		require.Equal(t, "new-name", got.Name)
	})

	t.Run("empty build-command is preserved to allow clearing", func(t *testing.T) {
		input := servicetypes.ServiceUpdateInput{
			BuildCommand:    pointers.From(""),
			ServiceIDOrName: "my-service-id",
		}
		got, err := servicetypes.NormalizeAndValidateUpdateInput(input)
		require.NoError(t, err)
		require.NotNil(t, got.BuildCommand)
		assert.Equal(t, "", *got.BuildCommand)
	})

	t.Run("whitespace-only build-command normalizes to empty string", func(t *testing.T) {
		input := servicetypes.ServiceUpdateInput{
			BuildCommand:    pointers.From("   "),
			ServiceIDOrName: "my-service-id",
		}
		got, err := servicetypes.NormalizeAndValidateUpdateInput(input)
		require.NoError(t, err)
		require.NotNil(t, got.BuildCommand)
		assert.Equal(t, "", *got.BuildCommand)
	})

	t.Run("whitespace-only name is treated as empty", func(t *testing.T) {
		input := servicetypes.ServiceUpdateInput{
			Name:            "   ",
			ServiceIDOrName: "my-service-id",
		}
		_, err := servicetypes.NormalizeAndValidateUpdateInput(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one update flag must be provided")
	})
}

func TestValidateForServiceType(t *testing.T) {
	tests := []struct {
		name        string
		input       servicetypes.ServiceUpdateInput
		serviceType servicetypes.ServiceType
		errMsg      string
	}{
		{
			name:        "health-check-path on cron job",
			input:       servicetypes.ServiceUpdateInput{HealthCheckPath: pointers.From("/health")},
			serviceType: servicetypes.ServiceTypeCronJob,
			errMsg:      "--health-check-path is not supported for cron_job",
		},
		{
			name:        "cron-schedule on web service",
			input:       servicetypes.ServiceUpdateInput{CronSchedule: pointers.From("* * * * *")},
			serviceType: servicetypes.ServiceTypeWebService,
			errMsg:      "--cron-schedule is not supported for web_service",
		},
		{
			name:        "publish-directory on private service",
			input:       servicetypes.ServiceUpdateInput{PublishDirectory: pointers.From("dist")},
			serviceType: servicetypes.ServiceTypePrivateService,
			errMsg:      "--publish-directory is not supported for private_service",
		},
		{
			name:        "start-command on static site",
			input:       servicetypes.ServiceUpdateInput{StartCommand: pointers.From("npm start")},
			serviceType: servicetypes.ServiceTypeStaticSite,
			errMsg:      "--start-command is not supported for static_site",
		},
		{
			name:        "maintenance-mode on private service",
			input:       servicetypes.ServiceUpdateInput{MaintenanceMode: pointers.From(true)},
			serviceType: servicetypes.ServiceTypePrivateService,
			errMsg:      "--maintenance-mode is not supported for private_service",
		},
		{
			name:        "num-instances on any type",
			input:       servicetypes.ServiceUpdateInput{NumInstances: pointers.From(3)},
			serviceType: servicetypes.ServiceTypeWebService,
			errMsg:      "--num-instances is not supported for update",
		},
		{
			name:        "plan on static site",
			input:       servicetypes.ServiceUpdateInput{Plan: pointers.From("pro")},
			serviceType: servicetypes.ServiceTypeStaticSite,
			errMsg:      "--plan is not supported for static_site",
		},
		{
			name:        "runtime on static site",
			input:       servicetypes.ServiceUpdateInput{Runtime: svcRuntime(servicetypes.ServiceRuntimeNode)},
			serviceType: servicetypes.ServiceTypeStaticSite,
			errMsg:      "--runtime is not supported for static_site",
		},
		{
			name:        "ip-allow-list on background worker",
			input:       servicetypes.ServiceUpdateInput{IPAllowList: []string{"cidr=10.0.0.0/8"}},
			serviceType: servicetypes.ServiceTypeBackgroundWorker,
			errMsg:      "--ip-allow-list is not supported for background_worker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.ValidateForServiceType(tt.serviceType)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}

	t.Run("allows valid flags for service type", func(t *testing.T) {
		input := servicetypes.ServiceUpdateInput{
			HealthCheckPath: pointers.From("/health"),
			Plan:            pointers.From("pro"),
		}
		err := input.ValidateForServiceType(servicetypes.ServiceTypeWebService)
		require.NoError(t, err)
	})

	t.Run("allows build-command on any type", func(t *testing.T) {
		input := servicetypes.ServiceUpdateInput{
			BuildCommand: pointers.From("npm ci"),
		}
		for _, st := range []servicetypes.ServiceType{
			servicetypes.ServiceTypeWebService,
			servicetypes.ServiceTypePrivateService,
			servicetypes.ServiceTypeBackgroundWorker,
			servicetypes.ServiceTypeCronJob,
			servicetypes.ServiceTypeStaticSite,
		} {
			err := input.ValidateForServiceType(st)
			require.NoError(t, err, "build-command should be allowed for %s", st)
		}
	})
}

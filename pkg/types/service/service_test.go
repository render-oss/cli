package service_test

import (
	"testing"

	"github.com/render-oss/cli/pkg/pointers"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceCreateCLIInputValidate(t *testing.T) {
	t.Run("non-interactive requires name", func(t *testing.T) {
		input := servicetypes.Service{Type: pointers.From("web_service")}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("non-interactive requires type when --from is not set", func(t *testing.T) {
		input := servicetypes.Service{Name: "my-service"}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type is required")
	})

	t.Run("interactive allows empty type", func(t *testing.T) {
		input := servicetypes.Service{Name: "my-service", Type: pointers.From("")}
		err := input.Validate(true)
		require.NoError(t, err)
	})

	t.Run("cannot specify both repo and image", func(t *testing.T) {
		input := servicetypes.Service{
			Name:  "my-service",
			Type:  pointers.From("web_service"),
			Repo:  pointers.From("https://github.com/org/repo"),
			Image: pointers.From("docker.io/image:tag"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify both")
	})

	t.Run("invalid type is rejected", func(t *testing.T) {
		input := servicetypes.Service{
			Name: "my-service",
			Type: pointers.From("invalid_type"),
			Repo: pointers.From("https://github.com/org/repo"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type must be one of")
	})

	t.Run("runtime is required when not image/from", func(t *testing.T) {
		input := servicetypes.Service{
			Name: "my-service",
			Type: pointers.From("web_service"),
			Repo: pointers.From("https://github.com/org/repo"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--runtime is required")
	})

	t.Run("static sites do not require runtime", func(t *testing.T) {
		input := servicetypes.Service{
			Name: "my-static-site",
			Type: pointers.From("static_site"),
			Repo: pointers.From("https://github.com/org/repo"),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("valid non-interactive input", func(t *testing.T) {
		input := servicetypes.Service{
			Name:         "my-service",
			Type:         pointers.From("web_service"),
			Repo:         pointers.From("https://github.com/org/repo"),
			Runtime:      pointers.From("node"),
			Region:       pointers.From("oregon"),
			BuildCommand: pointers.From("npm ci"),
			StartCommand: pointers.From("npm run start"),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("rejects malformed env var", func(t *testing.T) {
		input := servicetypes.Service{
			Name:         "my-service",
			Type:         pointers.From("web_service"),
			Repo:         pointers.From("https://github.com/org/repo"),
			Runtime:      pointers.From("node"),
			BuildCommand: pointers.From("npm ci"),
			StartCommand: pointers.From("npm run start"),
			EnvVars:      []string{"INVALID"},
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --env-var")
	})

	t.Run("rejects malformed secret file", func(t *testing.T) {
		input := servicetypes.Service{
			Name:         "my-service",
			Type:         pointers.From("web_service"),
			Repo:         pointers.From("https://github.com/org/repo"),
			Runtime:      pointers.From("node"),
			BuildCommand: pointers.From("npm ci"),
			StartCommand: pointers.From("npm run start"),
			SecretFiles:  []string{"INVALID_NO_COLON"},
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --secret-file")
	})

	t.Run("allows registry credential with image source", func(t *testing.T) {
		input := servicetypes.Service{
			Name:               "my-service",
			Type:               pointers.From("web_service"),
			Image:              pointers.From("docker.io/org/image:latest"),
			RegistryCredential: pointers.From("my-cred"),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("allows registry credential with docker runtime", func(t *testing.T) {
		input := servicetypes.Service{
			Name:               "my-service",
			Type:               pointers.From("web_service"),
			Repo:               pointers.From("https://github.com/org/repo"),
			Runtime:            pointers.From("docker"),
			RegistryCredential: pointers.From("my-cred"),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("rejects registry credential for native runtime", func(t *testing.T) {
		input := servicetypes.Service{
			Name:               "my-service",
			Type:               pointers.From("web_service"),
			Repo:               pointers.From("https://github.com/org/repo"),
			Runtime:            pointers.From("node"),
			BuildCommand:       pointers.From("npm ci"),
			StartCommand:       pointers.From("npm run start"),
			RegistryCredential: pointers.From("my-cred"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--registry-credential is only supported with --image or --runtime docker/image")
	})

	t.Run("cron job requires cron-command and cron-schedule when not cloning", func(t *testing.T) {
		input := servicetypes.Service{
			Name:         "my-cron",
			Type:         pointers.From("cron_job"),
			Repo:         pointers.From("https://github.com/org/repo"),
			Runtime:      pointers.From("node"),
			BuildCommand: pointers.From("npm ci"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cron-command and cron-schedule are required")
	})

	t.Run("cron job with --from skips initial cron-command requirement", func(t *testing.T) {
		// When cloning (--from), the initial cron field check is skipped because
		// the source service will provide those values. This test verifies that
		// using image runtime (which doesn't require build/start commands) passes.
		input := servicetypes.Service{
			Name:  "my-cron",
			Type:  pointers.From("cron_job"),
			From:  pointers.From("crn-abc123"),
			Image: pointers.From("docker.io/myimage:latest"),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("cron job with image runtime does not require build command", func(t *testing.T) {
		input := servicetypes.Service{
			Name:         "my-cron",
			Type:         pointers.From("cron_job"),
			Image:        pointers.From("docker.io/myimage:latest"),
			CronCommand:  pointers.From("echo hello"),
			CronSchedule: pointers.From("* * * * *"),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("missing repo or image returns clear error before cron validation", func(t *testing.T) {
		input := servicetypes.Service{
			Name:    "my-cron",
			Type:    pointers.From("cron_job"),
			Runtime: pointers.From("node"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		// Should get "repo or image required" not "cron-command required"
		assert.Contains(t, err.Error(), "either repo or image is required")
	})
}

func TestNormalizeAndValidateCreateInput(t *testing.T) {
	t.Run("normalizes and validates", func(t *testing.T) {
		input := servicetypes.Service{
			Name:         " my-service ",
			Type:         pointers.From("web_service"),
			Repo:         pointers.From("https://github.com/org/repo"),
			Runtime:      pointers.From("node"),
			BuildCommand: pointers.From("npm ci"),
			StartCommand: pointers.From("npm run start"),
		}

		got, err := servicetypes.NormalizeAndValidateCreateInput(input, false)
		require.NoError(t, err)
		require.Equal(t, "my-service", got.Name)
	})

	t.Run("returns validation error", func(t *testing.T) {
		input := servicetypes.Service{
			Name: "my-service",
			Type: pointers.From("web_service"),
			Repo: pointers.From("https://github.com/org/repo"),
		}

		_, err := servicetypes.NormalizeAndValidateCreateInput(input, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--runtime is required")
	})

	t.Run("whitespace-only required pointer fields normalize to nil and fail validation", func(t *testing.T) {
		input := servicetypes.Service{
			Name: "my-service",
			Type: pointers.From("   "),
			Repo: pointers.From("https://github.com/org/repo"),
		}

		_, err := servicetypes.NormalizeAndValidateCreateInput(input, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type is required")
	})

	t.Run("whitespace-only repo is treated as unset", func(t *testing.T) {
		input := servicetypes.Service{
			Name: "my-service",
			Type: pointers.From("web_service"),
			Repo: pointers.From("   "),
		}

		_, err := servicetypes.NormalizeAndValidateCreateInput(input, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "either repo or image is required")
	})
}

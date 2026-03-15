package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestServiceUpdateCmdArgsValidation(t *testing.T) {
	t.Run("rejects zero positional args", func(t *testing.T) {
		require.Error(t, ServiceUpdateCmd.Args(ServiceUpdateCmd, []string{}))
	})

	t.Run("accepts one positional arg", func(t *testing.T) {
		require.NoError(t, ServiceUpdateCmd.Args(ServiceUpdateCmd, []string{"my-service"}))
	})

	t.Run("rejects more than one positional arg", func(t *testing.T) {
		require.Error(t, ServiceUpdateCmd.Args(ServiceUpdateCmd, []string{"arg1", "arg2"}))
	})
}

func TestServiceUpdateAliasResolvesToUpdateCommand(t *testing.T) {
	plural, _, err := rootCmd.Find([]string{"services", "update"})
	require.NoError(t, err)
	require.Same(t, ServiceUpdateCmd, plural)

	alias, _, err := rootCmd.Find([]string{"service", "update"})
	require.NoError(t, err)
	require.Same(t, ServiceUpdateCmd, alias)
}

func TestServiceUpdateNoArgsValidationPreventsExecution(t *testing.T) {
	called := false
	cmd := &cobra.Command{
		Use:  "update",
		Args: ServiceUpdateCmd.Args,
		RunE: func(_ *cobra.Command, _ []string) error {
			called = true
			return nil
		},
	}
	cmd.SetArgs([]string{"arg1", "arg2"})

	err := cmd.Execute()
	require.Error(t, err)
	require.False(t, called)
}

func TestServiceUpdateFlagsRegistration(t *testing.T) {
	// Verify key flags are registered
	tests := []struct {
		flagName string
	}{
		{"name"},
		{"repo"},
		{"branch"},
		{"image"},
		{"plan"},
		{"runtime"},
		{"root-directory"},
		{"build-command"},
		{"start-command"},
		{"pre-deploy-command"},
		{"health-check-path"},
		{"publish-directory"},
		{"cron-command"},
		{"cron-schedule"},
		{"registry-credential"},
		{"auto-deploy"},
		{"build-filter-path"},
		{"build-filter-ignored-path"},
		{"num-instances"},
		{"max-shutdown-delay"},
		{"previews"},
		{"maintenance-mode"},
		{"maintenance-mode-uri"},
		{"ip-allow-list"},
	}

	for _, tt := range tests {
		t.Run(tt.flagName, func(t *testing.T) {
			flag := ServiceUpdateCmd.Flags().Lookup(tt.flagName)
			require.NotNil(t, flag, "flag %s should be registered", tt.flagName)
		})
	}
}

func TestServiceUpdateCommandStructure(t *testing.T) {
	t.Run("command use string is update [service]", func(t *testing.T) {
		require.Equal(t, "update [service]", ServiceUpdateCmd.Use)
	})

	t.Run("command requires exactly 1 positional arg", func(t *testing.T) {
		require.Error(t, ServiceUpdateCmd.Args(ServiceUpdateCmd, []string{}))
		require.NoError(t, ServiceUpdateCmd.Args(ServiceUpdateCmd, []string{"service"}))
		require.Error(t, ServiceUpdateCmd.Args(ServiceUpdateCmd, []string{"arg1", "arg2"}))
	})

	t.Run("command has RunE defined", func(t *testing.T) {
		require.NotNil(t, ServiceUpdateCmd.RunE)
	})
}

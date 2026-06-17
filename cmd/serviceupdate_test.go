package cmd

import (
	"testing"

	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func newServiceUpdateTestCmd() *cobra.Command {
	return newServiceUpdateCmd(dependencies.New(nil))
}

func TestServiceUpdateCmdArgsValidation(t *testing.T) {
	cmd := newServiceUpdateTestCmd()

	t.Run("rejects zero positional args", func(t *testing.T) {
		require.Error(t, cmd.Args(cmd, []string{}))
	})

	t.Run("accepts one positional arg", func(t *testing.T) {
		require.NoError(t, cmd.Args(cmd, []string{"my-service"}))
	})

	t.Run("rejects more than one positional arg", func(t *testing.T) {
		require.Error(t, cmd.Args(cmd, []string{"arg1", "arg2"}))
	})
}

func TestServiceUpdateAliasResolvesToUpdateCommand(t *testing.T) {
	root := newRootCmd()
	services := cobraServicesCommand()
	update := newServiceUpdateTestCmd()
	services.AddCommand(update)
	root.AddCommand(services)

	plural, _, err := root.Find([]string{"services", "update"})
	require.NoError(t, err)
	require.Same(t, update, plural)

	alias, _, err := root.Find([]string{"service", "update"})
	require.NoError(t, err)
	require.Same(t, update, alias)
}

func TestServiceUpdateNoArgsValidationPreventsExecution(t *testing.T) {
	update := newServiceUpdateTestCmd()
	called := false
	cmd := &cobra.Command{
		Use:  "update",
		Args: update.Args,
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
	cmd := newServiceUpdateTestCmd()

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
			flag := cmd.Flags().Lookup(tt.flagName)
			require.NotNil(t, flag, "flag %s should be registered", tt.flagName)
		})
	}
}

func TestServiceUpdateCommandStructure(t *testing.T) {
	cmd := newServiceUpdateTestCmd()

	t.Run("command use string is update <service>", func(t *testing.T) {
		require.Equal(t, "update <service>", cmd.Use)
	})

	t.Run("command requires exactly 1 positional arg", func(t *testing.T) {
		require.Error(t, cmd.Args(cmd, []string{}))
		require.NoError(t, cmd.Args(cmd, []string{"service"}))
		require.Error(t, cmd.Args(cmd, []string{"arg1", "arg2"}))
	})

	t.Run("command has RunE defined", func(t *testing.T) {
		require.NotNil(t, cmd.RunE)
	})
}

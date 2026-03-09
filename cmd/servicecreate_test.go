package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestServiceCreateCmdRejectsPositionalArgs(t *testing.T) {
	require.Error(t, ServiceCreateCmd.Args(ServiceCreateCmd, []string{"unexpected"}))
	require.NoError(t, ServiceCreateCmd.Args(ServiceCreateCmd, []string{}))
}

func TestServiceAliasResolvesToCreateCommand(t *testing.T) {
	plural, _, err := rootCmd.Find([]string{"services", "create"})
	require.NoError(t, err)
	require.Same(t, ServiceCreateCmd, plural)

	alias, _, err := rootCmd.Find([]string{"service", "create"})
	require.NoError(t, err)
	require.Same(t, ServiceCreateCmd, alias)
}

func TestServiceCreateNoArgsValidationPreventsExecution(t *testing.T) {
	called := false
	cmd := &cobra.Command{
		Use:  "create",
		Args: ServiceCreateCmd.Args,
		RunE: func(_ *cobra.Command, _ []string) error {
			called = true
			return nil
		},
	}
	cmd.SetArgs([]string{"unexpected"})

	err := cmd.Execute()
	require.Error(t, err)
	require.False(t, called)
}

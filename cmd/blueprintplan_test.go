package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/blueprint"
)

func TestBlueprintPlanHelp(t *testing.T) {
	server := renderapi.NewServer(t)

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "plan", "--help"})

	require.Equal(t, 0, result.ExitCode)

	require.Contains(t, output.Stdout, "plan <blueprintId>")
	require.Contains(t, output.Stdout, blueprint.ManualEnvVarsFileHelpPath)
	require.NotContains(t, output.Stdout, "--workspace")

	require.Empty(t, server.Requests)
}

func TestBlueprintPlanInputValidation(t *testing.T) {
	blueprintID := testids.BlueprintID("plan")
	for _, tc := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "valid Blueprint ID is ok",
			args:    []string{blueprintID},
			wantErr: "blueprint planning is not implemented yet",
		},
		{
			name:    "sync ID not accepted",
			args:    []string{testids.SyncID("plan")},
			wantErr: `invalid Blueprint ID "exe-`,
		},
		{
			name:    "missing Blueprint ID",
			wantErr: "accepts 1 arg(s), received 0",
		},
		{
			name:    "extra arguments",
			args:    []string{blueprintID, blueprintID},
			wantErr: "accepts 1 arg(s), received 2",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := renderapi.NewServer(t)
			args := append([]string{"ea", "blueprints", "plan", "--output", "json"}, tc.args...)

			result, output := executeBlueprintCommand(t, server, args)

			require.Equal(t, 1, result.ExitCode)
			require.Empty(t, output.Stdout)
			require.Contains(t, output.Stderr, tc.wantErr)
			require.Empty(t, server.Requests)
		})
	}
}

package blueprint_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/blueprint"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
)

func createResponseWithRequiredManualEnvVars() *bptypes.CreateBlueprintResponse {
	return &bptypes.CreateBlueprintResponse{
		Blueprint:       bptypes.Blueprint{Id: testids.BlueprintID("example")},
		PlanningOutcome: bptypes.PlanningOutcomePlanned,
		Sync: &bptypes.SyncWithPlan{RequiredManualEnvVars: []bptypes.RequiredManualEnvVar{
			{ServiceType: bptypes.ManualEnvVarTargetTypeWeb, ServiceName: "web", Key: "API_KEY"},
			{ServiceType: bptypes.ManualEnvVarTargetTypeWorker, ServiceName: "worker", Key: "API_KEY"},
		}},
	}
}

func TestPrepareManualEnvVarsFileHappyPath(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)

	filename, err := blueprint.PrepareManualEnvVarsFile(createResponseWithRequiredManualEnvVars())

	require.NoError(t, err)
	require.Equal(t, filepath.Join(configDir, "state", "manual-env-vars", testids.BlueprintID("example")+".json"), filename)

	content, err := os.ReadFile(filename)
	require.NoError(t, err)
	require.JSONEq(t, `[
  {"serviceType":"web","serviceName":"web","key":"API_KEY","value":""},
  {"serviceType":"worker","serviceName":"worker","key":"API_KEY","value":""}
]`, string(content))
}

func TestPrepareManualEnvVarsFileReplacesExistingFile(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
	filename := filepath.Join(configDir, "state", "manual-env-vars", testids.BlueprintID("example")+".json")
	require.NoError(t, os.MkdirAll(filepath.Dir(filename), 0o755))
	existing := []byte(`[{"serviceType":"web","serviceName":"web","key":"OLD_KEY","value":"old-value"}]`)
	require.NoError(t, os.WriteFile(filename, existing, 0o600))

	_, err := blueprint.PrepareManualEnvVarsFile(createResponseWithRequiredManualEnvVars())

	require.NoError(t, err)

	content, err := os.ReadFile(filename)
	require.NoError(t, err)
	require.NotContains(t, string(content), "OLD_KEY")
	require.NotContains(t, string(content), "old-value")
}

func TestPrepareManualEnvVarsFileRejectsUnexpectedBlueprintID(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
	response := createResponseWithRequiredManualEnvVars()
	response.Blueprint.Id = "../exs-example"

	filename, err := blueprint.PrepareManualEnvVarsFile(response)

	require.ErrorContains(t, err, `unexpected Blueprint ID "../exs-example"`)
	require.Empty(t, filename)

	files, err := os.ReadDir(configDir)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestPrepareManualEnvVarsFileNotNeeded(t *testing.T) {
	t.Run("writes no file when the plan does not need manual values", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			response bptypes.CreateBlueprintResponse
		}{
			{
				name: "plan has no manual values",
				response: bptypes.CreateBlueprintResponse{
					PlanningOutcome: bptypes.PlanningOutcomePlanned, Sync: &bptypes.SyncWithPlan{},
				},
			},
			{name: "no changes", response: bptypes.CreateBlueprintResponse{PlanningOutcome: bptypes.PlanningOutcomeNoChanges}},
			{
				name: "failed plan with required values",
				response: bptypes.CreateBlueprintResponse{
					PlanningOutcome: bptypes.PlanningOutcomeFailed,
					Sync:            &bptypes.SyncWithPlan{RequiredManualEnvVars: []bptypes.RequiredManualEnvVar{{Key: "API_KEY"}}},
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("RENDER_CLI_CONFIG_DIR", dir)

				filename, err := blueprint.PrepareManualEnvVarsFile(&tc.response)

				require.NoError(t, err)
				require.Empty(t, filename)

				files, err := os.ReadDir(dir)
				require.NoError(t, err)
				require.Empty(t, files)
			})
		}
	})
}

package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/files"
	"github.com/render-oss/cli/internal/testassert"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/blueprint"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
)

func TestBlueprintPlanTextOutput(t *testing.T) {
	server, _ := newBlueprintFixture(t)

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "plan", testBlueprintID, "--output", "text"})

	require.Equal(t, 0, result.ExitCode, output.Stderr)
	require.Empty(t, output.Stderr)
	testassert.ContainsInOrder(t, output.Stdout,
		"Sync exe-",
		"Planned, waiting for approval",
		"Review in Dashboard",
		"Next:",
		"render ea blueprints approve "+testBlueprintID,
	)
	require.Len(t, server.Requests, 1)
	require.Equal(t, "/blueprints/"+testBlueprintID+"/syncs", server.Requests[0].URI)
}

func TestBlueprintPlanPlanningOutcomes(t *testing.T) {
	errorNotices := []bptypes.BlueprintNotice{{Severity: bptypes.BlueprintNoticeSeverityError, Message: "Planning timed out. Re-plan to try again."}}
	for _, tc := range []struct {
		name         string
		response     bptypes.CreateBlueprintSyncResponse
		wantExitCode int
		wantNext     *string
	}{
		{
			name: "planned",
			response: bptypes.CreateBlueprintSyncResponse{
				PlanningOutcome: bptypes.PlanningOutcomePlanned, Notices: []bptypes.BlueprintNotice{},
				Sync: &bptypes.SyncWithPlan{Id: testids.SyncID("planned"), State: bptypes.SyncStateCreated},
			},
			wantExitCode: 0,
			wantNext:     new("render ea blueprints approve " + testBlueprintID),
		},
		{
			name: "no changes",
			response: bptypes.CreateBlueprintSyncResponse{
				PlanningOutcome: bptypes.PlanningOutcomeNoChanges, Notices: []bptypes.BlueprintNotice{},
			},
			wantExitCode: 0,
			wantNext:     new("Nothing to approve. If this is unexpected, push your Blueprint file changes to the Blueprint's branch, then re-plan with \"render ea blueprints plan " + testBlueprintID + "\"."),
		},
		{
			name: "unknown",
			response: bptypes.CreateBlueprintSyncResponse{
				PlanningOutcome: bptypes.PlanningOutcome("unknown"), Notices: []bptypes.BlueprintNotice{},
			},
			wantExitCode: 0,
			wantNext:     nil,
		},
		{
			name: "failed without a saved sync",
			response: bptypes.CreateBlueprintSyncResponse{
				PlanningOutcome: bptypes.PlanningOutcomeFailed, Notices: errorNotices,
			},
			wantExitCode: 1,
			wantNext:     new("fix the errors above, then run \"render ea blueprints plan " + testBlueprintID + "\""),
		},
		{
			name: "failed with a saved sync",
			response: bptypes.CreateBlueprintSyncResponse{
				PlanningOutcome: bptypes.PlanningOutcomeFailed, Notices: errorNotices,
				Sync: &bptypes.SyncWithPlan{Id: testids.SyncID("failed"), State: bptypes.SyncStateError},
			},
			wantExitCode: 1,
			wantNext:     new("fix the errors above, then run \"render ea blueprints plan " + testBlueprintID + "\""),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, manualEnvVarsFile := newBlueprintFixture(t)
			server.Blueprints.RespondWithBodyForPlan(tc.response)

			result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "plan", testBlueprintID, "--output", "json"})

			require.Equal(t, tc.wantExitCode, result.ExitCode)
			require.Empty(t, output.Stderr)
			var out blueprint.PlanOut
			require.NoError(t, json.Unmarshal([]byte(output.Stdout), &out))
			require.Equal(t, &tc.response, out.Data)
			require.Equal(t, tc.wantNext, out.Meta.Next)
			require.NoFileExists(t, manualEnvVarsFile)
		})
	}
}

func TestBlueprintPlanHTTPError(t *testing.T) {
	server, _ := newBlueprintFixture(t)
	server.Blueprints.RespondWithErrorForPlan(http.StatusConflict, "wait for the running sync to finish, then re-plan")

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "plan", testBlueprintID, "--output", "json"})

	require.Equal(t, 1, result.ExitCode)
	require.Empty(t, output.Stdout)
	require.Contains(t, output.Stderr, "wait for the running sync to finish, then re-plan")
}

func newSyncResponseWithRequiredManualEnvVars() bptypes.CreateBlueprintSyncResponse {
	return bptypes.CreateBlueprintSyncResponse{
		PlanningOutcome: bptypes.PlanningOutcomePlanned,
		Notices:         []bptypes.BlueprintNotice{},
		Sync: &bptypes.SyncWithPlan{RequiredManualEnvVars: []bptypes.RequiredManualEnvVar{
			{ServiceType: bptypes.ManualEnvVarTargetTypeWeb, ServiceName: "app", Key: "API_KEY"},
		}},
	}
}

func TestBlueprintPlanManualEnvVars(t *testing.T) {
	t.Run("writes manual env vars", func(t *testing.T) {
		server, manualEnvVarsFile := newBlueprintFixture(t)
		server.Blueprints.RespondWithBodyForPlan(newSyncResponseWithRequiredManualEnvVars())

		result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "plan", testBlueprintID, "--output", "json"})

		require.Equal(t, 0, result.ExitCode, output.Stderr)
		require.Empty(t, output.Stderr)
		require.FileExists(t, manualEnvVarsFile)
		var out blueprint.PlanOut
		require.NoError(t, json.Unmarshal([]byte(output.Stdout), &out))
		require.NotNil(t, out.Meta.Next)
		testassert.ContainsInOrder(t, *out.Meta.Next,
			manualEnvVarsFile+" lists these environment variables with empty values.",
			"then run \"render ea blueprints approve "+testBlueprintID+"\"",
		)
	})

	t.Run("prints results even after manual env vars file error", func(t *testing.T) {
		server, manualEnvVarsFile := newBlueprintFixture(t)
		response := newSyncResponseWithRequiredManualEnvVars()
		server.Blueprints.RespondWithBodyForPlan(response)
		// Make the state directory a regular file so creating
		// state/manual-env-vars fails after the sync is planned.
		stateDir := filepath.Dir(filepath.Dir(manualEnvVarsFile))
		require.NoError(t, os.WriteFile(stateDir, nil, 0o600))

		result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "plan", testBlueprintID, "--output", "json"})

		require.Equal(t, 1, result.ExitCode)
		require.Contains(t, output.Stderr, "write manual environment variables file")

		var out blueprint.PlanOut
		require.NoError(t, json.Unmarshal([]byte(output.Stdout), &out))
		require.Equal(t, &response, out.Data)
		require.Nil(t, out.Meta.Next)
	})

	// A newly planned sync must never leave approve reading values
	// written for an older sync.
	t.Run("manages manual env vars file differently based on planning outcome", func(t *testing.T) {
		const earlier = `[{"serviceType":"web","serviceName":"app","key":"OLD_KEY","value":"old-value"}]`
		for _, tc := range []struct {
			name         string
			response     bptypes.CreateBlueprintSyncResponse
			wantExitCode int
			// wantFile is the expected file contents after planning, or "" if the file should be removed.
			wantFile string
		}{
			{
				name: "planned without manual values removes it",
				response: bptypes.CreateBlueprintSyncResponse{
					PlanningOutcome: bptypes.PlanningOutcomePlanned, Notices: []bptypes.BlueprintNotice{},
					Sync: &bptypes.SyncWithPlan{Id: testids.SyncID("planned")},
				},
				wantExitCode: 0,
				wantFile:     "",
			},
			{
				name:         "planned with manual values replaces it",
				response:     newSyncResponseWithRequiredManualEnvVars(),
				wantExitCode: 0,
				wantFile:     `[{"serviceType":"web","serviceName":"app","key":"API_KEY","value":""}]`,
			},
			{
				name: "no changes keeps it",
				response: bptypes.CreateBlueprintSyncResponse{
					PlanningOutcome: bptypes.PlanningOutcomeNoChanges, Notices: []bptypes.BlueprintNotice{},
				},
				wantExitCode: 0,
				wantFile:     earlier,
			},
			{
				name: "failed keeps it",
				response: bptypes.CreateBlueprintSyncResponse{
					PlanningOutcome: bptypes.PlanningOutcomeFailed,
					Notices:         []bptypes.BlueprintNotice{{Severity: bptypes.BlueprintNoticeSeverityError, Message: "invalid plan"}},
				},
				wantExitCode: 1,
				wantFile:     earlier,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				server, manualEnvVarsFile := newBlueprintFixture(t)
				server.Blueprints.RespondWithBodyForPlan(tc.response)
				require.NoError(t, files.Write(manualEnvVarsFile, []byte(earlier)))

				result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "plan", testBlueprintID, "--output", "json"})

				require.Equal(t, tc.wantExitCode, result.ExitCode)
				require.Empty(t, output.Stderr)
				if tc.wantFile == "" {
					require.NoFileExists(t, manualEnvVarsFile)
					return
				}
				content, err := os.ReadFile(manualEnvVarsFile)
				require.NoError(t, err)
				require.JSONEq(t, tc.wantFile, string(content))
			})
		}
	})
}

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

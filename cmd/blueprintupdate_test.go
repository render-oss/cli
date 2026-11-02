package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/files"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/blueprint"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/command"
)

var testUpdateBlueprintID = testids.BlueprintID("update")

func TestBlueprintUpdateHelp(t *testing.T) {
	server := renderapi.NewServer(t)

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "update", "--help"})

	require.Equal(t, 0, result.ExitCode)
	require.Contains(t, output.Stdout, "update <blueprintId>")
	for _, flag := range []string{"--name <NAME>", "--branch <BRANCH>", "--path <PATH>", "--auto-sync"} {
		require.Contains(t, output.Stdout, flag)
	}
	require.NotContains(t, output.Stdout, "--repo")
	require.NotContains(t, output.Stdout, "--workspace")
	require.Empty(t, server.Requests)
}

func TestBlueprintUpdateWithoutPlanning(t *testing.T) {
	server, _ := newBlueprintFixture(t)

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "update", testBlueprintID, "--name", "renamed", "--auto-sync=false", "--output", "json"})

	require.Equal(t, 0, result.ExitCode, output.Stderr)
	require.Empty(t, output.Stderr)
	require.Len(t, server.Requests, 1)
	require.Equal(t, http.MethodPatch, server.Requests[0].Method)
	require.JSONEq(t, `{"name":"renamed","autoSync":false}`, string(server.Requests[0].Body))

	var out blueprint.UpdateOut
	require.NoError(t, json.Unmarshal([]byte(output.Stdout), &out))
	require.Equal(t, "renamed", out.Data.Blueprint.Name)
	require.False(t, out.Data.Blueprint.AutoSync)
	require.Nil(t, out.Data.PlanningOutcome)
	require.Nil(t, out.Data.Sync)
	require.Empty(t, out.Data.Notices)
	require.False(t, out.Meta.SyncRequested)
	require.Nil(t, out.Meta.Next)
}

func TestBlueprintUpdatePlanningOutcomes(t *testing.T) {
	replanNext := "fix the errors above, then run \"render ea blueprints plan " + testBlueprintID + "\""
	replanWithStaleSyncWarningNext := replanNext + ". Do not approve until re-planning succeeds. If you previously planned changes that are still waiting for approval, approving now could apply that earlier plan."
	for _, tc := range []struct {
		name string
		args []string
		// planResponse and planError queue the plan result; both nil uses the fake's planned sync.
		planResponse *bptypes.CreateBlueprintSyncResponse
		planError    string
		wantExitCode int
		wantOutcome  bptypes.PlanningOutcome
		wantNotices  []bptypes.BlueprintNotice
		wantNext     string
	}{
		{
			name:         "branch change is planned",
			args:         []string{"--branch", "release"},
			wantExitCode: 0,
			wantOutcome:  bptypes.PlanningOutcomePlanned,
			wantNotices:  []bptypes.BlueprintNotice{},
			wantNext:     "render ea blueprints approve " + testBlueprintID,
		},
		{
			name:         "unchanged path is still planned",
			args:         []string{"--path", "render.yaml"},
			wantExitCode: 0,
			wantOutcome:  bptypes.PlanningOutcomePlanned,
			wantNotices:  []bptypes.BlueprintNotice{},
			wantNext:     "render ea blueprints approve " + testBlueprintID,
		},
		{
			name: "no changes",
			args: []string{"--branch", "release"},
			planResponse: &bptypes.CreateBlueprintSyncResponse{
				PlanningOutcome: bptypes.PlanningOutcomeNoChanges, Notices: []bptypes.BlueprintNotice{},
			},
			wantExitCode: 0,
			wantOutcome:  bptypes.PlanningOutcomeNoChanges,
			wantNotices:  []bptypes.BlueprintNotice{},
			wantNext:     "No changes to approve. No further action is needed for this result. If you previously planned changes that are still waiting for approval, running \"render ea blueprints approve " + testBlueprintID + "\" now will approve that earlier plan. If you expected changes from this run, push your Blueprint file changes to the Blueprint's branch, then re-plan with \"render ea blueprints plan " + testBlueprintID + "\".",
		},
		{
			name: "planning failed without a saved sync",
			args: []string{"--branch", "release"},
			planResponse: &bptypes.CreateBlueprintSyncResponse{
				PlanningOutcome: bptypes.PlanningOutcomeFailed,
				Notices:         []bptypes.BlueprintNotice{{Severity: bptypes.BlueprintNoticeSeverityError, Message: "invalid plan"}},
			},
			wantExitCode: 1,
			wantOutcome:  bptypes.PlanningOutcomeFailed,
			wantNotices:  []bptypes.BlueprintNotice{{Severity: bptypes.BlueprintNoticeSeverityError, Message: "invalid plan"}},
			wantNext:     replanWithStaleSyncWarningNext,
		},
		{
			name: "planning failed with a saved sync",
			args: []string{"--branch", "release"},
			planResponse: &bptypes.CreateBlueprintSyncResponse{
				PlanningOutcome: bptypes.PlanningOutcomeFailed,
				Notices:         []bptypes.BlueprintNotice{{Severity: bptypes.BlueprintNoticeSeverityError, Message: "invalid plan"}},
				Sync:            &bptypes.SyncWithPlan{Id: testids.SyncID("failed"), State: bptypes.SyncStateError},
			},
			wantExitCode: 1,
			wantOutcome:  bptypes.PlanningOutcomeFailed,
			wantNotices:  []bptypes.BlueprintNotice{{Severity: bptypes.BlueprintNoticeSeverityError, Message: "invalid plan"}},
			wantNext:     replanNext,
		},
		{
			name:         "planning request error becomes an error notice",
			args:         []string{"--branch", "release"},
			planError:    "Wait for sync exe-running to finish, then re-plan.",
			wantExitCode: 1,
			wantOutcome:  bptypes.PlanningOutcomeFailed,
			wantNotices: []bptypes.BlueprintNotice{{
				Severity: bptypes.BlueprintNoticeSeverityError,
				Message:  "received response code 409: Wait for sync exe-running to finish, then re-plan.",
			}},
			wantNext: replanWithStaleSyncWarningNext,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := newBlueprintFixture(t)
			if tc.planResponse != nil {
				server.Blueprints.RespondWithBodyForPlan(*tc.planResponse)
			}
			if tc.planError != "" {
				server.Blueprints.RespondWithErrorForPlan(http.StatusConflict, tc.planError)
			}
			args := append([]string{"ea", "blueprints", "update", testBlueprintID, "--output", "json"}, tc.args...)

			result, output := executeBlueprintCommand(t, server, args)

			require.Equal(t, tc.wantExitCode, result.ExitCode, output.Stderr)
			require.Empty(t, output.Stderr)
			require.Len(t, server.Requests, 2)
			require.Equal(t, http.MethodPatch, server.Requests[0].Method)
			require.Equal(t, "/blueprints/"+testBlueprintID+"/syncs", server.Requests[1].URI)

			var out blueprint.UpdateOut
			require.NoError(t, json.Unmarshal([]byte(output.Stdout), &out))
			require.Equal(t, testBlueprintID, out.Data.Blueprint.Id)
			require.Equal(t, &tc.wantOutcome, out.Data.PlanningOutcome)
			require.Equal(t, tc.wantNotices, out.Data.Notices)
			require.True(t, out.Meta.SyncRequested)
			require.Equal(t, &tc.wantNext, out.Meta.Next)
		})
	}
}

func TestBlueprintUpdateSkipsPlanningWhenUpdateFails(t *testing.T) {
	server, _ := newBlueprintFixture(t)
	server.Blueprints.RespondWithErrorForUpdate(http.StatusBadRequest, "Could not read and validate the Blueprint file at the requested branch and path.")

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "update", testBlueprintID, "--branch", "missing", "--output", "json"})

	require.Equal(t, 1, result.ExitCode)
	require.Empty(t, output.Stdout)
	require.Contains(t, output.Stderr, "Could not read and validate the Blueprint file")
	require.Len(t, server.Requests, 1, "a failed update must not plan a sync")
}

func TestBlueprintUpdateReplacesManualEnvVarsFile(t *testing.T) {
	server, manualEnvVarsFile := newBlueprintFixture(t)
	server.Blueprints.RespondWithBodyForPlan(newSyncResponseWithRequiredManualEnvVars())
	require.NoError(t, files.Write(manualEnvVarsFile, []byte(`[{"serviceType":"web","serviceName":"app","key":"OLD_KEY","value":"old-value"}]`)))

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "update", testBlueprintID, "--branch", "release", "--path", "infra/render.yaml", "--output", "json"})

	require.Equal(t, 0, result.ExitCode, output.Stderr)
	require.Equal(t, http.MethodPatch, server.Requests[0].Method)
	require.JSONEq(t, `{"branch":"release","path":"infra/render.yaml"}`, string(server.Requests[0].Body))

	content, err := os.ReadFile(manualEnvVarsFile)
	require.NoError(t, err)
	require.JSONEq(t, `[{"serviceType":"web","serviceName":"app","key":"API_KEY","value":""}]`, string(content))

	var out blueprint.UpdateOut
	require.NoError(t, json.Unmarshal([]byte(output.Stdout), &out))
	require.NotNil(t, out.Meta.Next)
	require.Contains(t, *out.Meta.Next, manualEnvVarsFile+" lists these environment variables with empty values.")
}

func TestBlueprintUpdateStillPrintsResultWhenWritingManualEnvVarsFileFails(t *testing.T) {
	server, manualEnvVarsFile := newBlueprintFixture(t)
	server.Blueprints.RespondWithBodyForPlan(newSyncResponseWithRequiredManualEnvVars())
	// Make the state directory a regular file so creating
	// state/manual-env-vars fails after the sync is planned.
	stateDir := filepath.Dir(filepath.Dir(manualEnvVarsFile))
	require.NoError(t, os.WriteFile(stateDir, nil, 0o600))

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "update", testBlueprintID, "--branch", "release", "--output", "json"})

	require.Equal(t, 1, result.ExitCode)
	require.Contains(t, output.Stderr, "write manual environment variables file")

	var out blueprint.UpdateOut
	require.NoError(t, json.Unmarshal([]byte(output.Stdout), &out))
	require.Equal(t, "release", out.Data.Blueprint.Branch)
	require.Equal(t, new(bptypes.PlanningOutcomePlanned), out.Data.PlanningOutcome)
	require.True(t, out.Meta.SyncRequested)
	require.Nil(t, out.Meta.Next)
}

func TestBlueprintUpdateInputParsing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		want    blueprint.UpdateInput
		wantErr string
	}{
		{
			name: "only supplied settings are present",
			args: []string{testUpdateBlueprintID, "--branch", "release"},
			want: blueprint.UpdateInput{BlueprintID: testUpdateBlueprintID, Branch: new("release")},
		},
		{
			name: "all settings",
			args: []string{testUpdateBlueprintID, "--name", "app", "--branch", "release", "--path", "infra/render.yaml", "--auto-sync=true"},
			want: blueprint.UpdateInput{
				BlueprintID: testUpdateBlueprintID,
				Name:        new("app"),
				Branch:      new("release"),
				Path:        new("infra/render.yaml"),
				AutoSync:    new(true),
			},
		},
		{
			name: "disabling auto-sync is present",
			args: []string{testUpdateBlueprintID, "--auto-sync=false"},
			want: blueprint.UpdateInput{BlueprintID: testUpdateBlueprintID, AutoSync: new(false)},
		},
		{
			name:    "input is validated",
			args:    []string{testUpdateBlueprintID},
			wantErr: "provide at least one of --name, --branch, --path, or --auto-sync",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newBlueprintUpdateCmd(nil)
			cmd.SetContext(command.SetFormatInContext(context.Background(), new(command.TEXT)))

			require.NoError(t, cmd.ParseFlags(tc.args))

			var input blueprint.UpdateInput
			err := command.ParseCommand(cmd, cmd.Flags().Args(), &input)

			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, input)
		})
	}
}

func TestBlueprintUpdateRequiresOneBlueprintID(t *testing.T) {
	cmd := newBlueprintUpdateCmd(nil)
	require.Error(t, cmd.Args(cmd, nil))
	require.Error(t, cmd.Args(cmd, []string{testUpdateBlueprintID, testUpdateBlueprintID}))
}

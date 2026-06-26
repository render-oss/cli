package cmd

import (
	"encoding/json"
	"net/http"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeKVDelete runs `render kv delete <args>` against the fake server.
// Seeds and selects an active workspace before running the command.
func executeKVDelete(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()

	args := append([]string{"delete"}, extraArgs...)
	return executeKVCommand(t, server, args...)
}

func TestKVDelete_PreviewByID_DoesNotDelete(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "my-cache")

	result, err := executeKVDelete(t, server, kv.Id, "--output", "text")
	require.NoError(t, err)

	assert.Len(t, server.KV.Instances, 1, "preview must not delete")
	assert.Contains(t, result.Stdout, "would delete")
	assert.Contains(t, result.Stdout, "--confirm")
	assert.Contains(t, result.Stdout, kv.Id)
	assert.Contains(t, result.Stdout, "my-cache")
	assert.False(t, server.HasRequest("DELETE", "/key-value/"), "no DELETE call should be made in preview")
}

func TestKVDelete_ConfirmByID_Deletes(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "my-cache")

	result, err := executeKVDelete(t, server, kv.Id, "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Empty(t, server.KV.Instances)
	assert.Contains(t, result.Stdout, "Deleted")
	assert.Contains(t, result.Stdout, kv.Id)
}

func TestKVDelete_ConfirmByName_Deletes(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "by-name-cache")

	result, err := executeKVDelete(t, server, "by-name-cache", "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Empty(t, server.KV.Instances)
	assert.Contains(t, result.Stdout, "Deleted")
	assert.Contains(t, result.Stdout, kv.Id)
}

func TestKVDelete_NameCollision_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	seedKV(server, "key-value-not-unique-name")
	seedKV(server, "key-value-not-unique-name")

	_, err := executeKVDelete(t, server, "key-value-not-unique-name", "--confirm", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Multiple Key Value instances")
	assert.Len(t, server.KV.Instances, 2, "no delete on ambiguity")
	assert.False(t, server.HasRequest("DELETE", "/key-value/"))
}

func TestKVDelete_UnknownID_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	missing := testids.KeyValueID("missing")

	_, err := executeKVDelete(t, server, missing, "--confirm", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), missing)
	assert.Contains(t, err.Error(), "No Key Value with ID")
	assert.NotContains(t, err.Error(), "workspace", "ID errors should not mention workspace (IDs are global)")
	assert.False(t, server.HasRequest("DELETE", "/key-value/"))
}

func TestKVDelete_UnknownName_Errors(t *testing.T) {
	server := renderapi.NewServer(t)

	_, err := executeKVDelete(t, server, "does-not-exist", "--confirm", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
	assert.Contains(t, err.Error(), "Test Workspace", "name errors should surface the active workspace name")
	assert.Contains(t, err.Error(), kvTestWorkspaceID, "name errors should also include the workspace ID for copy-paste")
	assert.Contains(t, err.Error(), "render workspace set", "name errors should hint at the workspace-switch command")
}

func TestKVDelete_JSONOutput_AfterConfirm(t *testing.T) {
	server := renderapi.NewServer(t)
	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	env := project.Env("production")
	kv := seedKVInEnv(server, "json-cache", env.Id)

	result, err := executeKVDelete(t, server, kv.Id, "--confirm", "--output", "json")
	require.NoError(t, err)
	assert.Empty(t, server.KV.Instances)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	require.Len(t, body, 2)

	data := requireSubMap(t, body, "data")
	meta := requireSubMap(t, body, "meta")
	assert.Equal(t, kv.Id, data["id"])
	assert.Equal(t, "json-cache", data["name"])
	assert.Equal(t, kvTestWorkspaceID, data["ownerId"])
	assert.Equal(t, project.Project.Id, data["projectId"])
	assert.Equal(t, env.Id, data["environmentId"])
	assert.Equal(t, true, meta["deleted"])
	assert.NotContains(t, meta, "message")
	assert.NotContains(t, body, "keyValue")
	assert.NotContains(t, data, "projectName")
	assert.NotContains(t, data, "environmentName")
}

func TestKVDelete_JSONOutput_PreviewIncludesConfirmMessage(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "json-cache")

	result, err := executeKVDelete(t, server, kv.Id, "--output", "json")
	require.NoError(t, err)
	assert.Len(t, server.KV.Instances, 1, "preview must not delete")

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))

	data := requireSubMap(t, body, "data")
	meta := requireSubMap(t, body, "meta")
	assert.Equal(t, kv.Id, data["id"])
	assert.Equal(t, false, meta["deleted"])
	assert.Equal(t, "re-run with --confirm to delete", meta["message"])
}

func TestKVDelete_JSONOutput_OnError(t *testing.T) {
	// Errors are surfaced as plain text on stderr regardless of --output mode.
	// stdout stays empty so it can still be piped to jq without trailing usage spam.
	server := renderapi.NewServer(t)

	result, err := executeKVDelete(t, server, "does-not-exist", "--confirm", "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
	assert.Contains(t, result.Stderr, "does-not-exist")
	assert.Empty(t, result.Stdout, "stdout should be empty on error so JSON consumers don't choke on help text")
}

func TestKVDelete_NameCollision_NarrowedByEnvironment_Deletes(t *testing.T) {
	server := renderapi.NewServer(t)
	proj := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)

	prodKV := seedKVInEnv(server, "key-value-not-unique-name", proj.Env("production").Id)
	stagingKV := seedKVInEnv(server, "key-value-not-unique-name", proj.Env("staging").Id)

	result, err := executeKVDelete(t, server, "key-value-not-unique-name", "--environment", "production", "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "Deleted")
	assert.Contains(t, result.Stdout, prodKV.Id)
	require.Len(t, server.KV.Instances, 1, "only the production KV should be deleted")
	assert.Equal(t, stagingKV.Id, server.KV.Instances[0].Id)
}

func TestKVDelete_EnvironmentByID_Deletes(t *testing.T) {
	server := renderapi.NewServer(t)
	proj := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	kv := seedKVInEnv(server, "by-name-cache", proj.Env("production").Id)

	result, err := executeKVDelete(t, server, "by-name-cache", "--environment", proj.Env("production").Id, "--confirm", "--output", "text")
	require.NoError(t, err)
	assert.Empty(t, server.KV.Instances)
	assert.Contains(t, result.Stdout, "Deleted")
	assert.Contains(t, result.Stdout, kv.Id)
}

func TestKVDelete_UnknownEnvironment_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	seedKV(server, "any-cache")

	_, err := executeKVDelete(t, server, "any-cache", "--environment", "does-not-exist", "--confirm", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
	assert.Len(t, server.KV.Instances, 1, "no delete should occur if env resolution fails")
	assert.False(t, server.HasRequest("DELETE", "/key-value/"))
}

func TestKVDelete_IDWithMismatchedEnvironment_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	proj := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)

	kv := seedKVInEnv(server, "prod-cache", proj.Env("production").Id)

	_, err := executeKVDelete(t, server, kv.Id, "--environment", "staging", "--confirm", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), kv.Id)
	assert.Contains(t, err.Error(), "staging")
	assert.Len(t, server.KV.Instances, 1, "ID mismatch with --environment must not delete")
	assert.False(t, server.HasRequest("DELETE", "/key-value/"))
}

func TestKVDelete_IDLookup_NonNotFoundError_Surfaces(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "my-cache")

	// Inject a 500 on the next GET so the ID lookup fails for a reason
	// other than "not found". We expect that error to surface, not a
	// confusing "No Key Value named '...'" message from a silent fall
	// through to the list lookup.
	server.KV.RespondWith(http.StatusInternalServerError)

	_, err := executeKVDelete(t, server, kv.Id, "--confirm", "--output", "text")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "No Key Value named",
		"non-404 errors must not be hidden behind the name-fallback message")
	assert.Len(t, server.KV.Instances, 1, "no delete on lookup failure")
	assert.False(t, server.HasRequest("DELETE", "/key-value/"))
}

func TestKVDelete_DefaultOutput_TreatedAsText(t *testing.T) {
	// No --output flag set; the default ("interactive") should produce the
	// same human-readable preview as --output text rather than launching a TUI
	// or erroring out.
	server := renderapi.NewServer(t)
	kv := seedKV(server, "default-out")

	result, err := executeKVDelete(t, server, kv.Id)
	require.NoError(t, err)

	assert.Len(t, server.KV.Instances, 1, "default output should still preview, not delete")
	assert.Contains(t, result.Stdout, "would delete")
	assert.Contains(t, result.Stdout, kv.Id)
}

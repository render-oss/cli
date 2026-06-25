package cmd

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeKVGet runs `render ea kv get <args>` against the fake server.
// Seeds and selects an active workspace before running the command.
func executeKVGet(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()

	args := append([]string{"get"}, extraArgs...)
	return executeKVCommand(t, server, args...)
}

func TestKVGet_ByID_TextOutput(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "my-cache")

	result, err := executeKVGet(t, server, kv.Id, "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, kv.Id)
	assert.Contains(t, result.Stdout, "my-cache")
	assert.NotContains(t, result.Stdout, "CLI:", "connection info must not appear without --include-sensitive-connection-info")
	assert.False(t, server.HasRequest("GET", "/connection-info"), "no connection info request without flag")
}

func TestKVGet_TextOutput_IncludesWorkspaceProjectAndEnvironment(t *testing.T) {
	server := renderapi.NewServer(t)
	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	env := project.Env("production")
	kv := seedKVInEnv(server, "project-cache", env.Id)

	result, err := executeKVGet(t, server, kv.Id, "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "Workspace: Test Workspace ("+kvTestWorkspaceID+")")
	assert.Contains(t, result.Stdout, "Project: My Project ("+project.Project.Id+")")
	assert.Contains(t, result.Stdout, "Environment: production ("+env.Id+")")
}

func TestKVGet_ByName_TextOutput(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "by-name-cache")

	result, err := executeKVGet(t, server, "by-name-cache", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, kv.Id)
	assert.Contains(t, result.Stdout, "by-name-cache")
	assert.NotContains(t, result.Stdout, "CLI:")
}

func TestKVGet_WithConnectionInfo_ShowsCredentials(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "my-cache")

	result, err := executeKVGet(t, server, kv.Id, "--include-sensitive-connection-info", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, kv.Id)
	assert.Contains(t, result.Stdout, "CLI:")
	assert.Contains(t, result.Stdout, "Internal:")
	assert.Contains(t, result.Stdout, "External:")
	assert.True(t, server.HasRequest("GET", "/connection-info"))
}

func TestKVGet_WithEnvironmentFlag_ResolvesCorrectly(t *testing.T) {
	server := renderapi.NewServer(t)
	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)

	prodKV := seedKVInEnv(server, "shared-name", project.Env("production").Id)
	seedKVInEnv(server, "shared-name", project.Env("staging").Id)

	result, err := executeKVGet(t, server, "shared-name", "--environment", "production", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, prodKV.Id)
}

func TestKVGet_WithProjectFlag_NarrowsNameLookupToProject(t *testing.T) {
	server := renderapi.NewServer(t)

	projectA := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project A", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)
	projectB := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project B", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)

	projectAKV := seedKVInEnv(server, "cache", projectA.Env("staging").Id)
	projectBKV := seedKVInEnv(server, "cache", projectB.Env("production").Id)

	result, err := executeKVGet(t, server, "cache", "--project", "Project A", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, projectAKV.Id)
	assert.NotContains(t, result.Stdout, projectBKV.Id)
}

func TestKVGet_WithEnvironmentFlag_NarrowsLookupToActiveWorkspace(t *testing.T) {
	server := renderapi.NewServer(t)

	otherWorkspaceID := testids.WorkspaceID("other")
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: otherWorkspaceID, Name: "Other Workspace"}))

	activeProject := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Active Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	server.CreateProject(
		renderapi.ProjectAttrs{Name: "Other Project", OwnerId: otherWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)

	kv := seedKVInEnv(server, "cache", activeProject.Env("production").Id)

	result, err := executeKVGet(t, server, "cache", "--environment", "production", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, kv.Id)
}

func TestKVGet_IDWithMismatchedProject_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	otherProject := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Other Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)

	otherKV := seedKVInEnv(server, "other-cache", otherProject.Env("production").Id)

	_, err := executeKVGet(t, server, otherKV.Id, "--project", "My Project", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), otherKV.Id)
	assert.Contains(t, err.Error(), "My Project")
}

func TestKVGet_NameCollision_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	seedKV(server, "not-unique-name")
	seedKV(server, "not-unique-name")

	_, err := executeKVGet(t, server, "not-unique-name", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Multiple Key Value instances")
}

func TestKVGet_UnknownID_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	missing := testids.KeyValueID("missing")

	_, err := executeKVGet(t, server, missing, "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), missing)
	assert.Contains(t, err.Error(), "No Key Value with ID")
	assert.NotContains(t, err.Error(), "workspace", "ID errors should not mention workspace (IDs are global)")
}

func TestKVGet_UnknownName_Errors(t *testing.T) {
	server := renderapi.NewServer(t)

	_, err := executeKVGet(t, server, "does-not-exist", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
	assert.Contains(t, err.Error(), "Test Workspace", "name errors should surface the active workspace name")
	assert.Contains(t, err.Error(), kvTestWorkspaceID, "name errors should also include the workspace ID for copy-paste")
	assert.Contains(t, err.Error(), "render workspace set", "name errors should hint at the workspace-switch command")
}

func TestKVGet_JSONOutput_WithoutConnectionInfo(t *testing.T) {
	server := renderapi.NewServer(t)
	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	env := project.Env("production")
	kv := seedKVInEnv(server, "json-cache", env.Id)
	kv.Owner.Type = client.OwnerTypeTeam
	kv.Plan = client.KeyValuePlanStarter
	kv.Region = client.Oregon
	maxmemoryPolicy := "allkeys-lru"
	kv.Options.MaxmemoryPolicy = &maxmemoryPolicy
	kv.IpAllowList = []client.CidrBlockAndDescription{{
		CidrBlock:   "203.0.113.5/32",
		Description: "office",
	}}

	result, err := executeKVGet(t, server, kv.Id, "--output", "json")
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	data := requireSubMap(t, body, "data")
	assert.NotContains(t, body, "connectionInfo")
	assert.NotContains(t, data, "connectionInfo")
	assert.Equal(t, map[string]any{
		"id":              kv.Id,
		"name":            "json-cache",
		"plan":            string(client.KeyValuePlanStarter),
		"region":          string(client.Oregon),
		"status":          string(client.DatabaseStatusAvailable),
		"createdAt":       kv.CreatedAt.Format(time.RFC3339Nano),
		"updatedAt":       kv.UpdatedAt.Format(time.RFC3339Nano),
		"ownerId":         kvTestWorkspaceID,
		"ownerType":       string(client.OwnerTypeTeam),
		"projectId":       project.Project.Id,
		"environmentId":   env.Id,
		"ipAllowList":     []any{map[string]any{"cidrBlock": "203.0.113.5/32", "description": "office"}},
		"maxmemoryPolicy": maxmemoryPolicy,
	}, data)
}

func TestKVGet_JSONOutput_WithConnectionInfo(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "json-cache")

	result, err := executeKVGet(t, server, kv.Id, "--include-sensitive-connection-info", "--output", "json")
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	data := requireSubMap(t, body, "data")
	assert.Equal(t, kv.Id, data["id"])
	assert.Equal(t, "json-cache", data["name"])

	connectionInfo, ok := data["connectionInfo"].(map[string]any)
	require.True(t, ok, "expected connectionInfo object in JSON output: %s", result.Stdout)
	assert.Equal(t, map[string]any{
		"cliCommand":               "redis-cli -h fake-host -p 6379",
		"externalConnectionString": "rediss://fake-external",
		"internalConnectionString": "redis://fake-internal",
	}, connectionInfo)
}

func TestKVGet_IDLookup_NonNotFoundError_Surfaces(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "my-cache")

	// Inject a 500 on the next GET so the ID lookup fails for a non-404 reason.
	// We expect that error to surface, not a confusing "No Key Value named '...'" message.
	server.KV.RespondWith(http.StatusInternalServerError)

	_, err := executeKVGet(t, server, kv.Id, "--output", "text")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "No Key Value named",
		"non-404 errors must not be hidden behind the name-fallback message")
}

func TestKVGet_DefaultOutput_TreatedAsText(t *testing.T) {
	// No --output flag set; the default ("interactive") should produce the same
	// human-readable output as --output text rather than launching a TUI or erroring out.
	server := renderapi.NewServer(t)
	kv := seedKV(server, "default-out")

	result, err := executeKVGet(t, server, kv.Id)
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, kv.Id)
	assert.NotContains(t, result.Stdout, "CLI:")
}

package cmd

import (
	"encoding/json"
	"net/http"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeKVGet runs `render ea kv get <args>` against the fake server.
// Seeds and selects an active workspace before running the command.
func executeKVGet(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()
	t.Cleanup(resetKVGetFlags)
	resetKVGetFlags()

	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: ACTIVE_WORKSPACE_ID, Name: "Test Workspace"}))
	session := newCommandSession(t, server)
	if _, err := session.execute("workspace", "set", ACTIVE_WORKSPACE_ID, "--output", "text"); err != nil {
		return CommandResult{}, err
	}
	resetKVGetFlags()

	args := append([]string{"ea", "kv", "get"}, extraArgs...)
	return session.execute(args...)
}

// resetKVGetFlags resets the flags consumed by kvGetCmd between test runs,
// since Cobra retains values across Execute() calls.
func resetKVGetFlags() {
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "output" {
			f.Changed = false
			f.Value.Set(f.DefValue) //nolint:errcheck
		}
	})
	kvGetCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		f.Value.Set(f.DefValue) //nolint:errcheck
	})
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
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: ACTIVE_WORKSPACE_ID},
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
		renderapi.ProjectAttrs{Name: "Project A", OwnerId: ACTIVE_WORKSPACE_ID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)
	projectB := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project B", OwnerId: ACTIVE_WORKSPACE_ID},
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
		renderapi.ProjectAttrs{Name: "Active Project", OwnerId: ACTIVE_WORKSPACE_ID},
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
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: ACTIVE_WORKSPACE_ID},
		renderapi.EnvAttrs{Name: "production"},
	)
	otherProject := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Other Project", OwnerId: ACTIVE_WORKSPACE_ID},
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
	assert.Contains(t, err.Error(), ACTIVE_WORKSPACE_ID, "name errors should also include the workspace ID for copy-paste")
	assert.Contains(t, err.Error(), "render workspace set", "name errors should hint at the workspace-switch command")
}

func TestKVGet_JSONOutput_WithoutConnectionInfo(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "json-cache")

	result, err := executeKVGet(t, server, kv.Id, "--output", "json")
	require.NoError(t, err)

	var body struct {
		KeyValue struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"keyValue"`
		ConnectionInfo *struct{} `json:"connectionInfo"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	assert.Equal(t, kv.Id, body.KeyValue.ID)
	assert.Equal(t, "json-cache", body.KeyValue.Name)
	assert.Nil(t, body.ConnectionInfo)
}

func TestKVGet_JSONOutput_WithConnectionInfo(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "json-cache")

	result, err := executeKVGet(t, server, kv.Id, "--include-sensitive-connection-info", "--output", "json")
	require.NoError(t, err)

	var body struct {
		KeyValue struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"keyValue"`
		ConnectionInfo struct {
			CliCommand string `json:"cliCommand"`
		} `json:"connectionInfo"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	assert.Equal(t, kv.Id, body.KeyValue.ID)
	assert.Equal(t, "json-cache", body.KeyValue.Name)
	assert.NotEmpty(t, body.ConnectionInfo.CliCommand)
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

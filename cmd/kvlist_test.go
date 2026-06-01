package cmd

import (
	"encoding/json"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeKVList runs `render ea kv list <args>` against the fake server.
// Seeds and selects an active workspace before running the command.
func executeKVList(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()
	t.Cleanup(resetKVListFlags)
	resetKVListFlags()

	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: ACTIVE_WORKSPACE_ID, Name: "Test Workspace"}))
	session := newCommandSession(t, server)
	if _, err := session.execute("workspace", "set", ACTIVE_WORKSPACE_ID, "--output", "text"); err != nil {
		return CommandResult{}, err
	}
	resetKVListFlags()

	args := append([]string{"ea", "kv", "list"}, extraArgs...)
	return session.execute(args...)
}

func resetKVListFlags() {
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "output" {
			f.Changed = false
			f.Value.Set(f.DefValue) //nolint:errcheck
		}
	})
	kvListCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		f.Value.Set(f.DefValue) //nolint:errcheck
	})
}

func TestKVList_NoInstances(t *testing.T) {
	server := renderapi.NewServer(t)

	result, err := executeKVList(t, server, "--output", "text")
	require.NoError(t, err)
	assert.NotContains(t, result.Stdout, "red-")
}

func TestKVList_MultipleInstances(t *testing.T) {
	server := renderapi.NewServer(t)
	kv1 := seedKV(server, "cache-one")
	kv2 := seedKV(server, "cache-two")

	result, err := executeKVList(t, server, "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, kv1.Id)
	assert.Contains(t, result.Stdout, "cache-one")
	assert.Contains(t, result.Stdout, kv2.Id)
	assert.Contains(t, result.Stdout, "cache-two")
}

func TestKVList_FilterByEnvironmentID(t *testing.T) {
	server := renderapi.NewServer(t)
	proj := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: ACTIVE_WORKSPACE_ID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)

	prodKV := seedKVInEnv(server, "prod-cache", proj.Env("production").Id)
	stagingKV := seedKVInEnv(server, "staging-cache", proj.Env("staging").Id)

	result, err := executeKVList(t, server, "--environment", proj.Env("production").Id, "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, prodKV.Id)
	assert.NotContains(t, result.Stdout, stagingKV.Id)
}

func TestKVList_FilterByEnvironmentName(t *testing.T) {
	server := renderapi.NewServer(t)
	proj := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: ACTIVE_WORKSPACE_ID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)

	prodKV := seedKVInEnv(server, "prod-cache", proj.Env("production").Id)
	stagingKV := seedKVInEnv(server, "staging-cache", proj.Env("staging").Id)

	result, err := executeKVList(t, server, "--environment", "production", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, prodKV.Id)
	assert.NotContains(t, result.Stdout, stagingKV.Id)
}

func TestKVList_JSONOutput(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "json-cache")

	result, err := executeKVList(t, server, "--output", "json")
	require.NoError(t, err)

	var body []struct {
		KeyValue struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"keyValue"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	require.Len(t, body, 1)
	assert.Equal(t, kv.Id, body[0].KeyValue.ID)
	assert.Equal(t, "json-cache", body[0].KeyValue.Name)
}

func TestKVList_DefaultOutput_TreatedAsText(t *testing.T) {
	// No --output flag; the default ("interactive") should produce human-readable
	// text rather than launching a TUI.
	server := renderapi.NewServer(t)
	kv := seedKV(server, "default-out")

	result, err := executeKVList(t, server)
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, kv.Id)
	assert.Contains(t, result.Stdout, "default-out")
}

func TestKVList_UnknownEnvironment_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	seedKV(server, "any-cache")

	_, err := executeKVList(t, server, "--environment", "does-not-exist", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
}

func TestKVList_FilterByProject(t *testing.T) {
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

	aProdKV := seedKVInEnv(server, "a-prod-cache", projectA.Env("production").Id)
	aStagingKV := seedKVInEnv(server, "a-staging-cache", projectA.Env("staging").Id)
	bProdKV := seedKVInEnv(server, "b-prod-cache", projectB.Env("production").Id)

	result, err := executeKVList(t, server, "--project", "Project A", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, aProdKV.Id)
	assert.Contains(t, result.Stdout, aStagingKV.Id)
	assert.NotContains(t, result.Stdout, bProdKV.Id)
}

func TestKVList_FilterByProjectAndEnvironment_NarrowsToEnvWithinProject(t *testing.T) {
	server := renderapi.NewServer(t)

	projectA := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project A", OwnerId: ACTIVE_WORKSPACE_ID},
		renderapi.EnvAttrs{Name: "production"},
	)
	projectB := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project B", OwnerId: ACTIVE_WORKSPACE_ID},
		renderapi.EnvAttrs{Name: "production"},
	)

	aProdKV := seedKVInEnv(server, "a-prod-cache", projectA.Env("production").Id)
	bProdKV := seedKVInEnv(server, "b-prod-cache", projectB.Env("production").Id)

	result, err := executeKVList(t, server,
		"--project", "Project A",
		"--environment", "production",
		"--output", "text",
	)
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, aProdKV.Id)
	assert.NotContains(t, result.Stdout, bProdKV.Id)
}

func TestKVList_UnknownProject_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	seedKV(server, "any-cache")

	_, err := executeKVList(t, server, "--project", "does-not-exist", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
}

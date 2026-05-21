package cmd

import (
	"encoding/json"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
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
	projectID := testids.ProjectID("project")
	envProdID := testids.EnvironmentID("production")
	envStagingID := testids.EnvironmentID("staging")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: projectID, Name: "My Project", OwnerId: ACTIVE_WORKSPACE_ID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: envProdID, Name: "production", ProjectId: projectID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: envStagingID, Name: "staging", ProjectId: projectID}))

	prodKV := seedKVInEnv(server, "prod-cache", envProdID)
	stagingKV := seedKVInEnv(server, "staging-cache", envStagingID)

	result, err := executeKVList(t, server, "--environment", envProdID, "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, prodKV.Id)
	assert.NotContains(t, result.Stdout, stagingKV.Id)
}

func TestKVList_FilterByEnvironmentName(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("project")
	envProdID := testids.EnvironmentID("production")
	envStagingID := testids.EnvironmentID("staging")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: projectID, Name: "My Project", OwnerId: ACTIVE_WORKSPACE_ID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: envProdID, Name: "production", ProjectId: projectID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: envStagingID, Name: "staging", ProjectId: projectID}))

	prodKV := seedKVInEnv(server, "prod-cache", envProdID)
	stagingKV := seedKVInEnv(server, "staging-cache", envStagingID)

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

	projectAID := testids.ProjectID("project a")
	projectBID := testids.ProjectID("project b")
	projectAProdID := testids.EnvironmentID("project a production")
	projectAStagingID := testids.EnvironmentID("project a staging")
	projectBProdID := testids.EnvironmentID("project b production")

	projectA := renderapi.NewProject(renderapi.ProjectAttrs{Id: projectAID, Name: "Project A", OwnerId: ACTIVE_WORKSPACE_ID})
	projectA.EnvironmentIds = []string{projectAProdID, projectAStagingID}
	projectB := renderapi.NewProject(renderapi.ProjectAttrs{Id: projectBID, Name: "Project B", OwnerId: ACTIVE_WORKSPACE_ID})
	projectB.EnvironmentIds = []string{projectBProdID}

	server.Projects.Add(projectA)
	server.Projects.Add(projectB)
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: projectAProdID, Name: "production", ProjectId: projectAID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: projectAStagingID, Name: "staging", ProjectId: projectAID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: projectBProdID, Name: "production", ProjectId: projectBID}))

	aProdKV := seedKVInEnv(server, "a-prod-cache", projectAProdID)
	aStagingKV := seedKVInEnv(server, "a-staging-cache", projectAStagingID)
	bProdKV := seedKVInEnv(server, "b-prod-cache", projectBProdID)

	result, err := executeKVList(t, server, "--project", "Project A", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, aProdKV.Id)
	assert.Contains(t, result.Stdout, aStagingKV.Id)
	assert.NotContains(t, result.Stdout, bProdKV.Id)
}

func TestKVList_FilterByProjectAndEnvironment_NarrowsToEnvWithinProject(t *testing.T) {
	server := renderapi.NewServer(t)

	projectAID := testids.ProjectID("project a")
	projectBID := testids.ProjectID("project b")
	projectAProdID := testids.EnvironmentID("project a production")
	projectBProdID := testids.EnvironmentID("project b production")

	projectA := renderapi.NewProject(renderapi.ProjectAttrs{Id: projectAID, Name: "Project A", OwnerId: ACTIVE_WORKSPACE_ID})
	projectA.EnvironmentIds = []string{projectAProdID}
	projectB := renderapi.NewProject(renderapi.ProjectAttrs{Id: projectBID, Name: "Project B", OwnerId: ACTIVE_WORKSPACE_ID})
	projectB.EnvironmentIds = []string{projectBProdID}

	server.Projects.Add(projectA)
	server.Projects.Add(projectB)
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: projectAProdID, Name: "production", ProjectId: projectAID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: projectBProdID, Name: "production", ProjectId: projectBID}))

	aProdKV := seedKVInEnv(server, "a-prod-cache", projectAProdID)
	bProdKV := seedKVInEnv(server, "b-prod-cache", projectBProdID)

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

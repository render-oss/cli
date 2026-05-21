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

// executeKVResume runs `render ea kv resume <args>` against the fake server.
// Seeds and selects an active workspace before running the command.
func executeKVResume(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()
	t.Cleanup(resetKVResumeFlags)
	resetKVResumeFlags()

	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: ACTIVE_WORKSPACE_ID, Name: "Test Workspace"}))
	session := newCommandSession(t, server)
	if _, err := session.execute("workspace", "set", ACTIVE_WORKSPACE_ID, "--output", "text"); err != nil {
		return CommandResult{}, err
	}
	resetKVResumeFlags()

	args := append([]string{"ea", "kv", "resume"}, extraArgs...)
	return session.execute(args...)
}

func resetKVResumeFlags() {
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "output" {
			f.Changed = false
			f.Value.Set(f.DefValue) //nolint:errcheck
		}
	})
	kvResumeCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		f.Value.Set(f.DefValue) //nolint:errcheck
	})
}

// seedSuspendedKV adds a KV pre-seeded with Suspended status so resume tests
// can assert the status flips back to Available.
func seedSuspendedKV(server *renderapi.Server, name string) *client.KeyValueDetail {
	kv := seedKV(server, name)
	kv.Status = client.DatabaseStatusSuspended
	return kv
}

func TestKVResume_ByID_Resumes(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedSuspendedKV(server, "my-cache")

	result, err := executeKVResume(t, server, kv.Id, "--output", "text")
	require.NoError(t, err)

	assert.Equal(t, client.DatabaseStatusAvailable, server.KV.Instances[0].Status)
	assert.Contains(t, result.Stdout, "Resumed")
	assert.Contains(t, result.Stdout, kv.Id)
}

func TestKVResume_ByName_Resumes(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedSuspendedKV(server, "by-name-cache")

	result, err := executeKVResume(t, server, "by-name-cache", "--output", "text")
	require.NoError(t, err)

	assert.Equal(t, client.DatabaseStatusAvailable, server.KV.Instances[0].Status)
	assert.Contains(t, result.Stdout, "Resumed")
	assert.Contains(t, result.Stdout, kv.Id)
}

func TestKVResume_NameCollision_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	seedSuspendedKV(server, "key-value-not-unique-name")
	seedSuspendedKV(server, "key-value-not-unique-name")

	_, err := executeKVResume(t, server, "key-value-not-unique-name", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Multiple Key Value instances")
	for _, kv := range server.KV.Instances {
		assert.Equal(t, client.DatabaseStatusSuspended, kv.Status, "no resume on ambiguity")
	}
	assert.False(t, server.HasRequest("POST", "/key-value/"))
}

func TestKVResume_UnknownID_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	missing := testids.KeyValueID("missing")

	_, err := executeKVResume(t, server, missing, "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), missing)
	assert.Contains(t, err.Error(), "No Key Value with ID")
}

func TestKVResume_JSONOutput(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedSuspendedKV(server, "json-cache")

	result, err := executeKVResume(t, server, kv.Id, "--output", "json")
	require.NoError(t, err)
	assert.Equal(t, client.DatabaseStatusAvailable, server.KV.Instances[0].Status)

	var body struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	assert.Equal(t, kv.Id, body.ID)
	assert.Equal(t, "json-cache", body.Name)
	assert.Equal(t, string(client.DatabaseStatusAvailable), body.Status)
}

func TestKVResume_NameCollision_NarrowedByEnvironment_Resumes(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("project")
	envProdID := testids.EnvironmentID("production")
	envStagingID := testids.EnvironmentID("staging")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: projectID, Name: "My Project", OwnerId: ACTIVE_WORKSPACE_ID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: envProdID, Name: "production", ProjectId: projectID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: envStagingID, Name: "staging", ProjectId: projectID}))

	prodKV := seedKVInEnv(server, "key-value-not-unique-name", envProdID)
	prodKV.Status = client.DatabaseStatusSuspended
	stagingKV := seedKVInEnv(server, "key-value-not-unique-name", envStagingID)
	stagingKV.Status = client.DatabaseStatusSuspended

	result, err := executeKVResume(t, server, "key-value-not-unique-name", "--environment", "production", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "Resumed")
	assert.Contains(t, result.Stdout, prodKV.Id)
	for _, kv := range server.KV.Instances {
		if kv.Id == prodKV.Id {
			assert.Equal(t, client.DatabaseStatusAvailable, kv.Status)
		}
		if kv.Id == stagingKV.Id {
			assert.Equal(t, client.DatabaseStatusSuspended, kv.Status, "staging KV must not be resumed")
		}
	}
}

func TestKVResume_APIError_Surfaced(t *testing.T) {
	// First nextError is consumed by Resolve's GET; surface from there to
	// confirm failure propagates and no resume POST fires.
	server := renderapi.NewServer(t)
	kv := seedSuspendedKV(server, "my-cache")
	server.KV.RespondWith(http.StatusInternalServerError)

	_, err := executeKVResume(t, server, kv.Id, "--output", "text")
	require.Error(t, err)
	assert.Equal(t, client.DatabaseStatusSuspended, server.KV.Instances[0].Status, "API error must not flip status")
	assert.False(t, server.HasRequest("POST", "/key-value/"+kv.Id+"/resume"))
}

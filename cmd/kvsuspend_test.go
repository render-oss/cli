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

// executeKVSuspend runs `render ea kv suspend <args>` against the fake server.
// Seeds and selects an active workspace before running the command.
func executeKVSuspend(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()
	t.Cleanup(resetKVSuspendFlags)
	resetKVSuspendFlags()

	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: ACTIVE_WORKSPACE_ID, Name: "Test Workspace"}))
	session := newCommandSession(t, server)
	if _, err := session.execute("workspace", "set", ACTIVE_WORKSPACE_ID, "--output", "text"); err != nil {
		return CommandResult{}, err
	}
	resetKVSuspendFlags()

	args := append([]string{"ea", "kv", "suspend"}, extraArgs...)
	return session.execute(args...)
}

func resetKVSuspendFlags() {
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "confirm" || f.Name == "output" {
			f.Changed = false
			f.Value.Set(f.DefValue) //nolint:errcheck
		}
	})
	kvSuspendCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		f.Value.Set(f.DefValue) //nolint:errcheck
	})
}

func TestKVSuspend_PreviewByID_DoesNotSuspend(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "my-cache")

	result, err := executeKVSuspend(t, server, kv.Id, "--output", "text")
	require.NoError(t, err)

	assert.Equal(t, client.DatabaseStatusAvailable, server.KV.Instances[0].Status, "preview must not change status")
	assert.Contains(t, result.Stdout, "would suspend")
	assert.Contains(t, result.Stdout, "--confirm")
	assert.Contains(t, result.Stdout, kv.Id)
	assert.Contains(t, result.Stdout, "my-cache")
	assert.False(t, server.HasRequest("POST", "/key-value/"+kv.Id+"/suspend"), "no suspend call should be made in preview")
}

func TestKVSuspend_ConfirmByID_Suspends(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "my-cache")

	result, err := executeKVSuspend(t, server, kv.Id, "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Equal(t, client.DatabaseStatusSuspended, server.KV.Instances[0].Status)
	assert.Contains(t, result.Stdout, "Suspended")
	assert.Contains(t, result.Stdout, kv.Id)
}

func TestKVSuspend_ConfirmByName_Suspends(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "by-name-cache")

	result, err := executeKVSuspend(t, server, "by-name-cache", "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Equal(t, client.DatabaseStatusSuspended, server.KV.Instances[0].Status)
	assert.Contains(t, result.Stdout, "Suspended")
	assert.Contains(t, result.Stdout, kv.Id)
}

func TestKVSuspend_NameCollision_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	seedKV(server, "key-value-not-unique-name")
	seedKV(server, "key-value-not-unique-name")

	_, err := executeKVSuspend(t, server, "key-value-not-unique-name", "--confirm", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Multiple Key Value instances")
	for _, kv := range server.KV.Instances {
		assert.Equal(t, client.DatabaseStatusAvailable, kv.Status, "no suspend on ambiguity")
	}
	assert.False(t, server.HasRequest("POST", "/key-value/"))
}

func TestKVSuspend_UnknownID_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	missing := testids.KeyValueID("missing")

	_, err := executeKVSuspend(t, server, missing, "--confirm", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), missing)
	assert.Contains(t, err.Error(), "No Key Value with ID")
}

func TestKVSuspend_JSONOutput_AfterConfirm(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "json-cache")

	result, err := executeKVSuspend(t, server, kv.Id, "--confirm", "--output", "json")
	require.NoError(t, err)
	assert.Equal(t, client.DatabaseStatusSuspended, server.KV.Instances[0].Status)

	var body struct {
		KeyValue struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"keyValue"`
		Suspended bool `json:"suspended"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	assert.Equal(t, kv.Id, body.KeyValue.ID)
	assert.Equal(t, "json-cache", body.KeyValue.Name)
	assert.Equal(t, string(client.DatabaseStatusSuspended), body.KeyValue.Status)
	assert.True(t, body.Suspended)
}

func TestKVSuspend_NameCollision_NarrowedByEnvironment_Suspends(t *testing.T) {
	server := renderapi.NewServer(t)
	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: ACTIVE_WORKSPACE_ID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)

	prodKV := seedKVInEnv(server, "key-value-not-unique-name", project.Env("production").Id)
	stagingKV := seedKVInEnv(server, "key-value-not-unique-name", project.Env("staging").Id)

	result, err := executeKVSuspend(t, server, "key-value-not-unique-name", "--environment", "production", "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "Suspended")
	assert.Contains(t, result.Stdout, prodKV.Id)
	for _, kv := range server.KV.Instances {
		if kv.Id == prodKV.Id {
			assert.Equal(t, client.DatabaseStatusSuspended, kv.Status)
		}
		if kv.Id == stagingKV.Id {
			assert.Equal(t, client.DatabaseStatusAvailable, kv.Status, "staging KV must not be suspended")
		}
	}
}

func TestKVSuspend_APIError_Surfaced(t *testing.T) {
	// First nextError is consumed by Resolve's GET; surface the error from
	// there to confirm the failure path propagates and no suspend POST fires.
	server := renderapi.NewServer(t)
	kv := seedKV(server, "my-cache")
	server.KV.RespondWith(http.StatusInternalServerError)

	_, err := executeKVSuspend(t, server, kv.Id, "--confirm", "--output", "text")
	require.Error(t, err)
	assert.Equal(t, client.DatabaseStatusAvailable, server.KV.Instances[0].Status, "API error must not flip status")
	assert.False(t, server.HasRequest("POST", "/key-value/"+kv.Id+"/suspend"))
}

func TestKVSuspend_DefaultOutput_TreatedAsText(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "default-out")

	result, err := executeKVSuspend(t, server, kv.Id)
	require.NoError(t, err)

	assert.Equal(t, client.DatabaseStatusAvailable, server.KV.Instances[0].Status, "default output should still preview, not suspend")
	assert.Contains(t, result.Stdout, "would suspend")
	assert.Contains(t, result.Stdout, kv.Id)
}

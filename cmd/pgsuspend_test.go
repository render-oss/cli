package cmd

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
)

type pgSuspendHarness struct {
	t      *testing.T
	server *renderapi.Server
}

// newPGSuspendHarness sets up a server fake and seeds it with an (active) workspace.
func newPGSuspendHarness(t *testing.T) pgSuspendHarness {
	t.Helper()

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: pgActiveWorkspaceID, Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", pgActiveWorkspaceID)

	return pgSuspendHarness{t: t, server: server}
}

// execute invokes `render ea pg suspend`, passing through all extraArgs.
func (h pgSuspendHarness) execute(extraArgs ...string) (CommandResult, error) {
	h.t.Helper()

	return executePGCommand(h.t, h.server, append([]string{"ea", "pg", "suspend"}, extraArgs...)...)
}

func TestPGSuspend_PreviewByID_DoesNotSuspend(t *testing.T) {
	harness := newPGSuspendHarness(t)
	pg := seedPG(harness.server, "my-db")

	result, err := harness.execute(pg.Id, "--output", "text")
	require.NoError(t, err)

	assert.Equal(t, client.DatabaseStatusAvailable, harness.server.Postgres.Instances[0].Status, "preview must not change status")
	assert.Contains(t, result.Stdout, "would suspend")
	assert.Contains(t, result.Stdout, "--confirm")
	assert.Contains(t, result.Stdout, pg.Id)
	assert.Contains(t, result.Stdout, "my-db")
	assert.False(t, harness.server.HasRequest("POST", "/postgres/"+pg.Id+"/suspend"), "no suspend call should be made in preview")
}

func TestPGSuspend_ConfirmByID_Suspends(t *testing.T) {
	harness := newPGSuspendHarness(t)
	pg := seedPG(harness.server, "my-db")

	result, err := harness.execute(pg.Id, "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Equal(t, client.DatabaseStatusSuspended, harness.server.Postgres.Instances[0].Status)
	assert.Contains(t, result.Stdout, "Suspended")
	assert.Contains(t, result.Stdout, pg.Id)
}

func TestPGSuspend_ConfirmByName_Suspends(t *testing.T) {
	harness := newPGSuspendHarness(t)
	pg := seedPG(harness.server, "by-name-db")

	result, err := harness.execute("by-name-db", "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Equal(t, client.DatabaseStatusSuspended, harness.server.Postgres.Instances[0].Status)
	assert.Contains(t, result.Stdout, "Suspended")
	assert.Contains(t, result.Stdout, pg.Id)
}

func TestPGSuspend_NameCollision_Errors(t *testing.T) {
	harness := newPGSuspendHarness(t)
	seedPG(harness.server, "not-unique")
	seedPG(harness.server, "not-unique")

	_, err := harness.execute("not-unique", "--confirm", "--output", "text")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "Multiple Postgres databases")
	for _, pg := range harness.server.Postgres.Instances {
		assert.Equal(t, client.DatabaseStatusAvailable, pg.Status, "no suspend on ambiguity")
	}
	assert.False(t, harness.server.HasRequest("POST", "/postgres/"))
}

func TestPGSuspend_UnknownID_Errors(t *testing.T) {
	harness := newPGSuspendHarness(t)
	missing := testids.PostgresID("missing")

	_, err := harness.execute(missing, "--confirm", "--output", "text")
	require.Error(t, err)
	assert.ErrorContains(t, err, missing)
}

func TestPGSuspend_JSONOutput_AfterConfirm(t *testing.T) {
	harness := newPGSuspendHarness(t)
	pg := seedPG(harness.server, "json-db")

	result, err := harness.execute(pg.Id, "--confirm", "--output", "json")
	require.NoError(t, err)
	assert.Equal(t, client.DatabaseStatusSuspended, harness.server.Postgres.Instances[0].Status)

	var body struct {
		Postgres struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"postgres"`
		Suspended bool `json:"suspended"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	assert.Equal(t, pg.Id, body.Postgres.ID)
	assert.Equal(t, "json-db", body.Postgres.Name)
	assert.Equal(t, string(client.DatabaseStatusSuspended), body.Postgres.Status)
	assert.True(t, body.Suspended)
}

func TestPGSuspend_NameCollision_NarrowedByEnvironment_Suspends(t *testing.T) {
	harness := newPGSuspendHarness(t)
	proj := harness.server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)

	prodPG := seedPGInEnv(harness.server, "not-unique", proj.Env("production").Id)
	stagingPG := seedPGInEnv(harness.server, "not-unique", proj.Env("staging").Id)

	result, err := harness.execute("not-unique", "--environment", "production", "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "Suspended")
	assert.Contains(t, result.Stdout, prodPG.Id)
	assert.Equal(t, client.DatabaseStatusSuspended, prodPG.Status)
	assert.Equal(t, client.DatabaseStatusAvailable, stagingPG.Status, "staging database must not be suspended")
}

func TestPGSuspend_APIError_Surfaced(t *testing.T) {
	// First nextError is consumed by Resolve's GET; surface the error from there
	// to confirm the failure path propagates and no suspend POST fires.
	harness := newPGSuspendHarness(t)
	pg := seedPG(harness.server, "my-db")
	harness.server.Postgres.RespondWith(http.StatusInternalServerError)

	_, err := harness.execute(pg.Id, "--confirm", "--output", "text")
	require.Error(t, err)
	assert.Equal(t, client.DatabaseStatusAvailable, harness.server.Postgres.Instances[0].Status, "API error must not flip status")
	assert.False(t, harness.server.HasRequest("POST", "/postgres/"+pg.Id+"/suspend"))
}

func TestPGSuspend_DefaultOutput_TreatedAsText(t *testing.T) {
	harness := newPGSuspendHarness(t)
	pg := seedPG(harness.server, "default-out")

	result, err := harness.execute(pg.Id)
	require.NoError(t, err)

	assert.Equal(t, client.DatabaseStatusAvailable, harness.server.Postgres.Instances[0].Status, "default output should still preview, not suspend")
	assert.Contains(t, result.Stdout, "would suspend")
	assert.Contains(t, result.Stdout, pg.Id)
}

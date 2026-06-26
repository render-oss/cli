package cmd

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
)

type pgResumeHarness struct {
	t      *testing.T
	server *renderapi.Server
}

// newPGResumeHarness sets up a server fake and seeds it with an (active) workspace.
func newPGResumeHarness(t *testing.T) pgResumeHarness {
	t.Helper()

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: pgActiveWorkspaceID, Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", pgActiveWorkspaceID)

	return pgResumeHarness{t: t, server: server}
}

// execute invokes `render pg resume`, passing through all extraArgs.
func (h pgResumeHarness) execute(extraArgs ...string) (CommandResult, error) {
	h.t.Helper()

	return executePGCommand(h.t, h.server, append([]string{"pg", "resume"}, extraArgs...)...)
}

// seedSuspendedPG seeds a Postgres pre-set to Suspended so resume tests can assert
// the status flips back to Available.
func seedSuspendedPG(server *renderapi.Server, name string) *client.PostgresDetail {
	pg := seedPG(server, name)
	pg.Status = client.DatabaseStatusSuspended
	return pg
}

func TestPGResume_ByID_Resumes(t *testing.T) {
	harness := newPGResumeHarness(t)
	pg := seedSuspendedPG(harness.server, "my-db")

	result, err := harness.execute(pg.Id, "--output", "text")
	require.NoError(t, err)

	assert.Equal(t, client.DatabaseStatusAvailable, harness.server.Postgres.Instances[0].Status)
	assert.Contains(t, result.Stdout, "Resumed")
	assert.Contains(t, result.Stdout, pg.Id)
}

func TestPGResume_ByName_Resumes(t *testing.T) {
	harness := newPGResumeHarness(t)
	pg := seedSuspendedPG(harness.server, "by-name-db")

	result, err := harness.execute("by-name-db", "--output", "text")
	require.NoError(t, err)

	assert.Equal(t, client.DatabaseStatusAvailable, harness.server.Postgres.Instances[0].Status)
	assert.Contains(t, result.Stdout, "Resumed")
	assert.Contains(t, result.Stdout, pg.Id)
}

func TestPGResume_NameCollision_Errors(t *testing.T) {
	harness := newPGResumeHarness(t)
	seedSuspendedPG(harness.server, "not-unique")
	seedSuspendedPG(harness.server, "not-unique")

	_, err := harness.execute("not-unique", "--output", "text")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "Multiple Postgres databases")
	for _, pg := range harness.server.Postgres.Instances {
		assert.Equal(t, client.DatabaseStatusSuspended, pg.Status, "no resume on ambiguity")
	}
	assert.False(t, harness.server.HasRequest("POST", "/postgres/"))
}

func TestPGResume_UnknownID_Errors(t *testing.T) {
	harness := newPGResumeHarness(t)
	missing := testids.PostgresID("missing")

	_, err := harness.execute(missing, "--output", "text")
	require.Error(t, err)
	assert.ErrorContains(t, err, missing)
}

func TestPGResume_JSONOutput(t *testing.T) {
	harness := newPGResumeHarness(t)
	pg := seedSuspendedPG(harness.server, "json-db")

	result, err := harness.execute(pg.Id, "--output", "json")
	require.NoError(t, err)
	assert.Equal(t, client.DatabaseStatusAvailable, harness.server.Postgres.Instances[0].Status)

	body := unmarshalPGJSONOutput(t, result.Stdout)
	data := testrequire.SubMap(t, body, "data")
	assert.Equal(t, pg.Id, data["id"])
	assert.Equal(t, "json-db", data["name"])
	assert.Equal(t, string(client.DatabaseStatusAvailable), data["status"])
}

func TestPGResume_NameCollision_NarrowedByEnvironment_Resumes(t *testing.T) {
	harness := newPGResumeHarness(t)
	proj := harness.server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)

	prodPG := seedPGInEnv(harness.server, "not-unique", proj.Env("production").Id)
	prodPG.Status = client.DatabaseStatusSuspended
	stagingPG := seedPGInEnv(harness.server, "not-unique", proj.Env("staging").Id)
	stagingPG.Status = client.DatabaseStatusSuspended

	result, err := harness.execute("not-unique", "--environment", "production", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "Resumed")
	assert.Contains(t, result.Stdout, prodPG.Id)
	assert.Equal(t, client.DatabaseStatusAvailable, prodPG.Status)
	assert.Equal(t, client.DatabaseStatusSuspended, stagingPG.Status, "staging database must not be resumed")
}

func TestPGResume_APIError_Surfaced(t *testing.T) {
	// First nextError is consumed by Resolve's GET; surface the error from there
	// to confirm the failure path propagates and no resume POST fires.
	harness := newPGResumeHarness(t)
	pg := seedSuspendedPG(harness.server, "my-db")
	harness.server.Postgres.RespondWith(http.StatusInternalServerError)

	_, err := harness.execute(pg.Id, "--output", "text")
	require.Error(t, err)
	assert.Equal(t, client.DatabaseStatusSuspended, harness.server.Postgres.Instances[0].Status, "API error must not flip status")
	assert.False(t, harness.server.HasRequest("POST", "/postgres/"+pg.Id+"/resume"))
}

func TestPGResume_DefaultOutput_TreatedAsText(t *testing.T) {
	harness := newPGResumeHarness(t)
	pg := seedSuspendedPG(harness.server, "default-out")

	result, err := harness.execute(pg.Id)
	require.NoError(t, err)

	assert.Equal(t, client.DatabaseStatusAvailable, harness.server.Postgres.Instances[0].Status)
	assert.Contains(t, result.Stdout, "Resumed")
	assert.Contains(t, result.Stdout, pg.Id)
}

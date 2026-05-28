package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
)

type pgGetHarness struct {
	t      *testing.T
	server *renderapi.Server
}

// newPGGetHarness sets up a server fake and seeds it with an (active) workspace
func newPGGetHarness(t *testing.T) pgGetHarness {
	t.Helper()

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: pgActiveWorkspaceID, Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", pgActiveWorkspaceID)

	return pgGetHarness{t: t, server: server}
}

// execute invokes the `render ea pg get` command, passing through all extraArgs
func (h pgGetHarness) execute(extraArgs ...string) (CommandResult, error) {
	h.t.Helper()

	return executePGCommand(h.t, h.server, append([]string{"ea", "pg", "get"}, extraArgs...)...)
}

func TestPGGet_ByID(t *testing.T) {
	harness := newPGGetHarness(t)
	pg := seedPG(harness.server, "my-db")

	result, err := harness.execute(pg.Id, "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, pg.Id)
	assert.Contains(t, result.Stdout, "my-db")
	assert.NotContains(t, result.Stdout, "PSQL:", "connection info must not appear without --include-sensitive-connection-info")
	assert.False(t, harness.server.HasRequest("GET", "/connection-info"), "no connection info request without flag")
}

func TestPGGet_ByName(t *testing.T) {
	harness := newPGGetHarness(t)
	pg := seedPG(harness.server, "by-name-db")

	result, err := harness.execute("by-name-db", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, pg.Id)
	assert.Contains(t, result.Stdout, "by-name-db")
	assert.NotContains(t, result.Stdout, "PSQL:")
}

func TestPGGet_WithConnectionInfo_ShowsCredentials(t *testing.T) {
	harness := newPGGetHarness(t)
	pg := seedPG(harness.server, "my-db")

	result, err := harness.execute(pg.Id, "--include-sensitive-connection-info", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, pg.Id)
	assert.Contains(t, result.Stdout, "PSQL:")
	assert.Contains(t, result.Stdout, "Internal:")
	assert.Contains(t, result.Stdout, "External:")
	assert.Contains(t, result.Stdout, "Password:")
	assert.True(t, harness.server.HasRequest("GET", "/connection-info"))
}

func TestPGGet_WithEnvironmentFlag_ResolvesCorrectly(t *testing.T) {
	harness := newPGGetHarness(t)
	projectID := testids.ProjectID("project")
	prodEnvId := testids.EnvironmentID("production")
	stagingEnvId := testids.EnvironmentID("staging")
	harness.server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: projectID, Name: "My Project", OwnerId: pgActiveWorkspaceID}))
	harness.server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: prodEnvId, Name: "production", ProjectId: projectID}))
	harness.server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: stagingEnvId, Name: "staging", ProjectId: projectID}))

	prodPG := seedPGInEnv(harness.server, "shared-name", prodEnvId)
	stagingPG := seedPGInEnv(harness.server, "shared-name", stagingEnvId)

	result, err := harness.execute("shared-name", "--environment", "production", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, prodPG.Id)
	assert.NotContains(t, result.Stdout, stagingPG.Id)
}

func TestPGGet_WithProjectFlag_NarrowsNameLookupToProject(t *testing.T) {
	harness := newPGGetHarness(t)

	projectAID := testids.ProjectID("project-a")
	projectBID := testids.ProjectID("project-b")
	projectAEnvID := testids.EnvironmentID("project-a-production")
	projectBEnvID := testids.EnvironmentID("project-b-production")
	projectA := renderapi.NewProject(renderapi.ProjectAttrs{Id: projectAID, Name: "Project A", OwnerId: pgActiveWorkspaceID})
	projectA.EnvironmentIds = []string{projectAEnvID}
	projectB := renderapi.NewProject(renderapi.ProjectAttrs{Id: projectBID, Name: "Project B", OwnerId: pgActiveWorkspaceID})
	projectB.EnvironmentIds = []string{projectBEnvID}
	harness.server.Projects.Add(projectA)
	harness.server.Projects.Add(projectB)
	harness.server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: projectAEnvID, Name: "production", ProjectId: projectAID}))
	harness.server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: projectBEnvID, Name: "production", ProjectId: projectBID}))

	projectAPG := seedPGInEnv(harness.server, "db", projectAEnvID)
	projectBPG := seedPGInEnv(harness.server, "db", projectBEnvID)

	result, err := harness.execute("db", "--project", "Project A", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, projectAPG.Id)
	assert.NotContains(t, result.Stdout, projectBPG.Id)
}

func TestPGGet_IDWithMismatchedProject_Errors(t *testing.T) {
	harness := newPGGetHarness(t)
	projectID := testids.ProjectID("project")
	otherProjectID := testids.ProjectID("other-project")
	projectEnvID := testids.EnvironmentID("project")
	otherEnvID := testids.EnvironmentID("other-project")
	project := renderapi.NewProject(renderapi.ProjectAttrs{Id: projectID, Name: "My Project", OwnerId: pgActiveWorkspaceID})
	project.EnvironmentIds = []string{projectEnvID}
	otherProject := renderapi.NewProject(renderapi.ProjectAttrs{Id: otherProjectID, Name: "Other Project", OwnerId: pgActiveWorkspaceID})
	otherProject.EnvironmentIds = []string{otherEnvID}
	harness.server.Projects.Add(project)
	harness.server.Projects.Add(otherProject)
	harness.server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: projectEnvID, Name: "production", ProjectId: projectID}))
	harness.server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: otherEnvID, Name: "production", ProjectId: otherProjectID}))

	otherPG := seedPGInEnv(harness.server, "other-db", otherEnvID)

	_, err := harness.execute(otherPG.Id, "--project", "My Project", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), otherPG.Id)
	assert.Contains(t, err.Error(), "My Project")
}

func TestPGGet_NameCollision_Errors(t *testing.T) {
	harness := newPGGetHarness(t)
	seedPG(harness.server, "not-unique-name")
	seedPG(harness.server, "not-unique-name")

	_, err := harness.execute("not-unique-name", "--output", "text")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "Multiple Postgres databases")
}

func TestPGGet_UnknownID_Errors(t *testing.T) {
	harness := newPGGetHarness(t)
	missing := testids.PostgresID("missing")

	_, err := harness.execute(missing, "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), missing)
	assert.Contains(t, err.Error(), "No Postgres database with ID")
	assert.NotContains(t, err.Error(), "workspace", "ID errors should not mention workspace (IDs are global)")
	assert.NotContains(t, err.Error(), pgActiveWorkspaceID)
}

func TestPGGet_UnknownName_Errors(t *testing.T) {
	harness := newPGGetHarness(t)

	_, err := harness.execute("does-not-exist", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
	assert.Contains(t, err.Error(), pgActiveWorkspaceID, "name errors should also include the workspace ID for copy-paste")
	assert.Contains(t, err.Error(), "render workspace set", "name errors should hint at the workspace-switch command")
}

func TestPGGet_JSONOutput_WithoutConnectionInfo(t *testing.T) {
	harness := newPGGetHarness(t)
	pg := seedPG(harness.server, "json-db")

	result, err := harness.execute(pg.Id, "--output", "json")
	require.NoError(t, err)

	var body struct {
		Postgres struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"postgres"`
		ConnectionInfo *struct{} `json:"connectionInfo"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	assert.Equal(t, pg.Id, body.Postgres.ID)
	assert.Equal(t, "json-db", body.Postgres.Name)
	assert.Nil(t, body.ConnectionInfo)
	assert.NotContains(t, result.Stdout, "psqlCommand", "connection info must not appear without --include-sensitive-connection-info")
	assert.NotContains(t, result.Stdout, "password", "connection info must not appear without --include-sensitive-connection-info")
	assert.False(t, harness.server.HasRequest("GET", "/connection-info"), "no connection info request without flag")
}

func TestPGGet_JSONOutput_WithConnectionInfo(t *testing.T) {
	harness := newPGGetHarness(t)
	pg := seedPG(harness.server, "json-db")

	result, err := harness.execute(pg.Id, "--include-sensitive-connection-info", "--output", "json")
	require.NoError(t, err)

	var body struct {
		Postgres struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"postgres"`
		ConnectionInfo struct {
			PsqlCommand string `json:"psqlCommand"`
			Password    string `json:"password"`
		} `json:"connectionInfo"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	assert.Equal(t, pg.Id, body.Postgres.ID)
	assert.Equal(t, "json-db", body.Postgres.Name)
	assert.NotEmpty(t, body.ConnectionInfo.PsqlCommand)
	assert.NotEmpty(t, body.ConnectionInfo.Password)
}

func TestPGGet_DefaultOutput_TreatedAsText(t *testing.T) {
	harness := newPGGetHarness(t)
	pg := seedPG(harness.server, "default-out")

	result, err := harness.execute(pg.Id)
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, pg.Id)
	assert.NotContains(t, result.Stdout, "PSQL:")
}

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

type pgListHarness struct {
	t      *testing.T
	server *renderapi.Server
}

// newPGListHarness sets up a server fake and seeds it with an (active) workspace
func newPGListHarness(t *testing.T) pgListHarness {
	t.Helper()

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: pgActiveWorkspaceID, Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", pgActiveWorkspaceID)

	return pgListHarness{t: t, server: server}
}

// execute invokes the `render ea pg list` command, passing through all extraArgs
func (h pgListHarness) execute(extraArgs ...string) (CommandResult, error) {
	h.t.Helper()

	return executePGCommand(h.t, h.server, append([]string{"ea", "pg", "list"}, extraArgs...)...)
}

func TestPGList_NoDatabases(t *testing.T) {
	harness := newPGListHarness(t)

	result, err := harness.execute("--output", "text")
	require.NoError(t, err)

	assert.NotContains(t, result.Stdout, "dpg-")
	assert.Contains(t, result.Stdout, "No Postgres databases")

	result, err = harness.execute("--output", "json")
	require.NoError(t, err)

	var body []struct {
		Postgres struct {
			ID string `json:"id"`
		} `json:"postgres"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	assert.Empty(t, body)
}

func TestPGList_MultipleDatabases(t *testing.T) {
	harness := newPGListHarness(t)
	pg1 := seedPG(harness.server, "db-one")
	pg2 := seedPG(harness.server, "db-two")

	result, err := harness.execute("--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, pg1.Id)
	assert.Contains(t, result.Stdout, "db-one")
	assert.Contains(t, result.Stdout, pg2.Id)
	assert.Contains(t, result.Stdout, "db-two")
}

func TestPGList_FilterByEnvironmentName(t *testing.T) {
	harness := newPGListHarness(t)
	projectID := testids.ProjectID("project")
	envProdID := testids.EnvironmentID("production")
	envStagingID := testids.EnvironmentID("staging")
	harness.server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: projectID, Name: "My Project", OwnerId: pgActiveWorkspaceID}))
	harness.server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: envProdID, Name: "production", ProjectId: projectID}))
	harness.server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: envStagingID, Name: "staging", ProjectId: projectID}))

	prodPG := seedPGInEnv(harness.server, "prod-db", envProdID)
	stagingPG := seedPGInEnv(harness.server, "staging-db", envStagingID)

	result, err := harness.execute("--environment", "production", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, prodPG.Id)
	assert.NotContains(t, result.Stdout, stagingPG.Id)
}

func TestPGList_FilterByProject(t *testing.T) {
	harness := newPGListHarness(t)

	projectAID := testids.ProjectID("project-a")
	projectBID := testids.ProjectID("project-b")
	projectAProdID := testids.EnvironmentID("project-a-production")
	projectAStagingID := testids.EnvironmentID("project-a-staging")
	projectBProdID := testids.EnvironmentID("project-b-production")

	projectA := renderapi.NewProject(renderapi.ProjectAttrs{Id: projectAID, Name: "Project A", OwnerId: pgActiveWorkspaceID})
	projectA.EnvironmentIds = []string{projectAProdID, projectAStagingID}
	projectB := renderapi.NewProject(renderapi.ProjectAttrs{Id: projectBID, Name: "Project B", OwnerId: pgActiveWorkspaceID})
	projectB.EnvironmentIds = []string{projectBProdID}

	harness.server.Projects.Add(projectA)
	harness.server.Projects.Add(projectB)
	harness.server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: projectAProdID, Name: "production", ProjectId: projectAID}))
	harness.server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: projectAStagingID, Name: "staging", ProjectId: projectAID}))
	harness.server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: projectBProdID, Name: "production", ProjectId: projectBID}))

	projectAProdPG := seedPGInEnv(harness.server, "a-prod-db", projectAProdID)
	projectAStagingPG := seedPGInEnv(harness.server, "a-staging-db", projectAStagingID)
	projectBProdPG := seedPGInEnv(harness.server, "b-prod-db", projectBProdID)

	result, err := harness.execute("--project", "Project A", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, projectAProdPG.Id)
	assert.Contains(t, result.Stdout, projectAStagingPG.Id)
	assert.NotContains(t, result.Stdout, projectBProdPG.Id)
}

func TestPGList_FilterByProjectAndEnvironment_NarrowsToEnvWithinProject(t *testing.T) {
	harness := newPGListHarness(t)

	projectAID := testids.ProjectID("project-a")
	projectBID := testids.ProjectID("project-b")
	projectAProdID := testids.EnvironmentID("project-a-production")
	projectBProdID := testids.EnvironmentID("project-b-production")

	projectA := renderapi.NewProject(renderapi.ProjectAttrs{Id: projectAID, Name: "Project A", OwnerId: pgActiveWorkspaceID})
	projectA.EnvironmentIds = []string{projectAProdID}
	projectB := renderapi.NewProject(renderapi.ProjectAttrs{Id: projectBID, Name: "Project B", OwnerId: pgActiveWorkspaceID})
	projectB.EnvironmentIds = []string{projectBProdID}

	harness.server.Projects.Add(projectA)
	harness.server.Projects.Add(projectB)
	harness.server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: projectAProdID, Name: "production", ProjectId: projectAID}))
	harness.server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: projectBProdID, Name: "production", ProjectId: projectBID}))

	projectAProdPG := seedPGInEnv(harness.server, "a-prod-db", projectAProdID)
	projectBProdPG := seedPGInEnv(harness.server, "b-prod-db", projectBProdID)

	result, err := harness.execute(
		"--project", "Project A",
		"--environment", "production",
		"--output", "text",
	)
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, projectAProdPG.Id)
	assert.NotContains(t, result.Stdout, projectBProdPG.Id)
}

func TestPGList_JSONOutput(t *testing.T) {
	harness := newPGListHarness(t)
	pg1 := seedPG(harness.server, "json-db-one")
	pg2 := seedPG(harness.server, "json-db-two")

	result, err := harness.execute("--output", "json")
	require.NoError(t, err)

	var body []struct {
		Postgres struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"postgres"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	require.Len(t, body, 2)

	// Build an _un-ordered_ object to assert against
	// Order is just determined by our fake render api, so not meaningful
	got := map[string]string{}
	for _, item := range body {
		got[item.Postgres.ID] = item.Postgres.Name
	}
	assert.Equal(t, map[string]string{
		pg1.Id: "json-db-one",
		pg2.Id: "json-db-two",
	}, got)
}

func TestPGList_DefaultOutput_TreatedAsText(t *testing.T) {
	harness := newPGListHarness(t)
	pg := seedPG(harness.server, "default-out")

	result, err := harness.execute()
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, pg.Id)
	assert.Contains(t, result.Stdout, "default-out")
}

func TestPGList_UnknownProject_Errors(t *testing.T) {
	harness := newPGListHarness(t)
	seedPG(harness.server, "any-db")

	_, err := harness.execute("--project", "does-not-exist", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
}

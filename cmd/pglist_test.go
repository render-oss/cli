package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testrequire"
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

	body := unmarshalPGJSONOutput(t, result.Stdout)
	assert.Empty(t, testrequire.SubSlice(t, body, "data"))
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
	project := harness.server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)

	prodPG := seedPGInEnv(harness.server, "prod-db", project.Env("production").Id)
	stagingPG := seedPGInEnv(harness.server, "staging-db", project.Env("staging").Id)

	result, err := harness.execute("--environment", "production", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, prodPG.Id)
	assert.NotContains(t, result.Stdout, stagingPG.Id)
}

func TestPGList_FilterByProject(t *testing.T) {
	harness := newPGListHarness(t)

	projectA := harness.server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project A", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)
	projectB := harness.server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project B", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)

	projectAProdPG := seedPGInEnv(harness.server, "a-prod-db", projectA.Env("production").Id)
	projectAStagingPG := seedPGInEnv(harness.server, "a-staging-db", projectA.Env("staging").Id)
	projectBProdPG := seedPGInEnv(harness.server, "b-prod-db", projectB.Env("production").Id)

	result, err := harness.execute("--project", "Project A", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, projectAProdPG.Id)
	assert.Contains(t, result.Stdout, projectAStagingPG.Id)
	assert.NotContains(t, result.Stdout, projectBProdPG.Id)
}

func TestPGList_FilterByProjectAndEnvironment_NarrowsToEnvWithinProject(t *testing.T) {
	harness := newPGListHarness(t)

	projectA := harness.server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project A", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	projectB := harness.server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project B", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)

	projectAProdPG := seedPGInEnv(harness.server, "a-prod-db", projectA.Env("production").Id)
	projectBProdPG := seedPGInEnv(harness.server, "b-prod-db", projectB.Env("production").Id)

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

	body := unmarshalPGJSONOutput(t, result.Stdout)
	data := testrequire.SubSlice(t, body, "data")
	require.Len(t, data, 2)

	// Build an _un-ordered_ object to assert against
	// Order is just determined by our fake render api, so not meaningful
	got := map[string]string{}
	for _, item := range data {
		itemMap, ok := item.(map[string]any)
		require.True(t, ok)
		got[itemMap["id"].(string)] = itemMap["name"].(string)
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

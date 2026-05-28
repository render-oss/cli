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

func executePGDelete(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: pgActiveWorkspaceID, Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", pgActiveWorkspaceID)

	return executePGCommand(t, server, append([]string{"ea", "pg", "delete"}, extraArgs...)...)
}

func TestPGDelete_PreviewByID_DoesNotDelete(t *testing.T) {
	server := renderapi.NewServer(t)
	pg := seedPG(server, "my-db")

	result, err := executePGDelete(t, server, pg.Id, "--output", "text")
	require.NoError(t, err)

	assert.Len(t, server.Postgres.Instances, 1)
	assert.Contains(t, result.Stdout, "would delete")
	assert.Contains(t, result.Stdout, "--confirm")
	assert.Contains(t, result.Stdout, pg.Id)
	assert.Contains(t, result.Stdout, "my-db")
	assert.False(t, server.HasRequest("DELETE", "/postgres/"))
}

func TestPGDelete_ConfirmByID_Deletes(t *testing.T) {
	server := renderapi.NewServer(t)
	pg := seedPG(server, "my-db")

	result, err := executePGDelete(t, server, pg.Id, "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Empty(t, server.Postgres.Instances)
	assert.Contains(t, result.Stdout, "Deleted")
	assert.Contains(t, result.Stdout, pg.Id)
}

func TestPGDelete_ConfirmByName_Deletes(t *testing.T) {
	server := renderapi.NewServer(t)
	pg := seedPG(server, "by-name-db")

	result, err := executePGDelete(t, server, "by-name-db", "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Empty(t, server.Postgres.Instances)
	assert.Contains(t, result.Stdout, "Deleted")
	assert.Contains(t, result.Stdout, pg.Id)
}

func TestPGDelete_NameCollision_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	seedPG(server, "not-unique")
	seedPG(server, "not-unique")

	_, err := executePGDelete(t, server, "not-unique", "--confirm", "--output", "text")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "Multiple Postgres databases")
	assert.Len(t, server.Postgres.Instances, 2)
	assert.False(t, server.HasRequest("DELETE", "/postgres/"))
}

func TestPGDelete_JSONOutput_AfterConfirm(t *testing.T) {
	server := renderapi.NewServer(t)
	pg := seedPG(server, "json-db")

	result, err := executePGDelete(t, server, pg.Id, "--confirm", "--output", "json")
	require.NoError(t, err)
	assert.Empty(t, server.Postgres.Instances)

	var body struct {
		Postgres struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"postgres"`
		Deleted bool `json:"deleted"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	assert.Equal(t, pg.Id, body.Postgres.ID)
	assert.Equal(t, "json-db", body.Postgres.Name)
	assert.True(t, body.Deleted)
}

func TestPGDelete_JSONOutput_OnError(t *testing.T) {
	server := renderapi.NewServer(t)

	result, err := executePGDelete(t, server, "does-not-exist", "--confirm", "--output", "json")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "does-not-exist")
	assert.Contains(t, result.Stderr, "does-not-exist")
	assert.Empty(t, result.Stdout)
}

func TestPGDelete_NameCollision_NarrowedByEnvironment_Deletes(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("project")
	envProdID := testids.EnvironmentID("production")
	envStagingID := testids.EnvironmentID("staging")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: projectID, Name: "My Project", OwnerId: pgActiveWorkspaceID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: envProdID, Name: "production", ProjectId: projectID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: envStagingID, Name: "staging", ProjectId: projectID}))

	prodPG := seedPGInEnv(server, "not-unique", envProdID)
	stagingPG := seedPGInEnv(server, "not-unique", envStagingID)

	result, err := executePGDelete(t, server, "not-unique", "--environment", "production", "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "Deleted")
	assert.Contains(t, result.Stdout, prodPG.Id)
	require.Len(t, server.Postgres.Instances, 1)
	assert.Equal(t, stagingPG.Id, server.Postgres.Instances[0].Id)
}

func TestPGDelete_NameCollision_NarrowedByProject_Deletes(t *testing.T) {
	server := renderapi.NewServer(t)
	projectAID := testids.ProjectID("project-a")
	projectBID := testids.ProjectID("project-b")
	envAID := testids.EnvironmentID("project-a-production")
	envBID := testids.EnvironmentID("project-b-production")
	projectA := renderapi.NewProject(renderapi.ProjectAttrs{Id: projectAID, Name: "Project A", OwnerId: pgActiveWorkspaceID})
	projectA.EnvironmentIds = []string{envAID}
	projectB := renderapi.NewProject(renderapi.ProjectAttrs{Id: projectBID, Name: "Project B", OwnerId: pgActiveWorkspaceID})
	projectB.EnvironmentIds = []string{envBID}
	server.Projects.Add(projectA)
	server.Projects.Add(projectB)
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: envAID, Name: "production", ProjectId: projectAID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: envBID, Name: "production", ProjectId: projectBID}))

	projectAPG := seedPGInEnv(server, "not-unique", envAID)
	projectBPG := seedPGInEnv(server, "not-unique", envBID)

	result, err := executePGDelete(t, server, "not-unique", "--project", "Project A", "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "Deleted")
	assert.Contains(t, result.Stdout, projectAPG.Id)
	require.Len(t, server.Postgres.Instances, 1)
	assert.Equal(t, projectBPG.Id, server.Postgres.Instances[0].Id)
}

func TestPGDelete_IDWithMismatchedEnvironment_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("project")
	envProdID := testids.EnvironmentID("production")
	envStagingID := testids.EnvironmentID("staging")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: projectID, Name: "My Project", OwnerId: pgActiveWorkspaceID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: envProdID, Name: "production", ProjectId: projectID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: envStagingID, Name: "staging", ProjectId: projectID}))
	pg := seedPGInEnv(server, "prod-db", envProdID)

	_, err := executePGDelete(t, server, pg.Id, "--environment", "staging", "--confirm", "--output", "text")
	require.Error(t, err)

	assert.Contains(t, err.Error(), pg.Id)
	assert.Contains(t, err.Error(), "staging")
	assert.Len(t, server.Postgres.Instances, 1)
	assert.False(t, server.HasRequest("DELETE", "/postgres/"))
}

func TestPGDelete_DefaultOutput_TreatedAsText(t *testing.T) {
	server := renderapi.NewServer(t)
	pg := seedPG(server, "default-out")

	result, err := executePGDelete(t, server, pg.Id)
	require.NoError(t, err)

	assert.Len(t, server.Postgres.Instances, 1)
	assert.Contains(t, result.Stdout, "would delete")
	assert.Contains(t, result.Stdout, pg.Id)
}

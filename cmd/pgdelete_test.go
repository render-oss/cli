package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
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
	proj := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)

	prodPG := seedPGInEnv(server, "not-unique", proj.Env("production").Id)
	stagingPG := seedPGInEnv(server, "not-unique", proj.Env("staging").Id)

	result, err := executePGDelete(t, server, "not-unique", "--environment", "production", "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "Deleted")
	assert.Contains(t, result.Stdout, prodPG.Id)
	require.Len(t, server.Postgres.Instances, 1)
	assert.Equal(t, stagingPG.Id, server.Postgres.Instances[0].Id)
}

func TestPGDelete_NameCollision_NarrowedByProject_Deletes(t *testing.T) {
	server := renderapi.NewServer(t)
	projectA := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project A", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	projectB := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project B", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)

	projectAPG := seedPGInEnv(server, "not-unique", projectA.Env("production").Id)
	projectBPG := seedPGInEnv(server, "not-unique", projectB.Env("production").Id)

	result, err := executePGDelete(t, server, "not-unique", "--project", "Project A", "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "Deleted")
	assert.Contains(t, result.Stdout, projectAPG.Id)
	require.Len(t, server.Postgres.Instances, 1)
	assert.Equal(t, projectBPG.Id, server.Postgres.Instances[0].Id)
}

func TestPGDelete_IDWithMismatchedEnvironment_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	proj := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)
	pg := seedPGInEnv(server, "prod-db", proj.Env("production").Id)

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

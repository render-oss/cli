package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var serviceTestWorkspaceID = testids.WorkspaceID("active")

const serviceTestWorkspaceName = "Test Workspace"

func seedService(server *renderapi.Server, name string) *client.Service {
	return server.Services.Add(renderapi.NewWebService(renderapi.WebServiceAttrs{
		Service: renderapi.CommonServiceAttrs{
			Name:    name,
			OwnerID: serviceTestWorkspaceID,
		},
	}))
}

func executeServiceDelete(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()
	return executeServiceDeleteWithConfig(t, server, &config.Config{
		Workspace:     serviceTestWorkspaceID,
		WorkspaceName: serviceTestWorkspaceName,
	}, extraArgs...)
}

func executeServiceDeleteWithoutActiveWorkspace(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()
	return executeServiceDeleteWithConfig(t, server, &config.Config{}, extraArgs...)
}

func executeServiceDeleteWithConfig(t *testing.T, server *renderapi.Server, cfg *config.Config, extraArgs ...string) (CommandResult, error) {
	t.Helper()

	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: serviceTestWorkspaceID, Name: serviceTestWorkspaceName}))
	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_HOST", server.URL())
	t.Setenv("RENDER_API_KEY", "test-api-key")
	t.Setenv("RENDER_WORKSPACE", "")
	if cfg != nil {
		require.NoError(t, cfg.Persist())
	}

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{
			StdinTTY:  false,
			StdoutTTY: false,
			StderrTTY: false,
		}, nil
	}

	root := newRootCmd()
	services := cobraServicesCommand()
	services.AddCommand(newServiceDeleteCmd(deps))
	root.AddCommand(services)
	setupRootCmdPersistentRun(root, deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(append([]string{"services", "delete"}, extraArgs...))

	execErr := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, execErr
}

func cobraServicesCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "services",
		Aliases: []string{"service"},
	}
}

func TestServiceDelete_PreviewByID_DoesNotDelete(t *testing.T) {
	server := renderapi.NewServer(t)
	svc := seedService(server, "my-api")

	result, err := executeServiceDelete(t, server, svc.Id, "--output", "text")
	require.NoError(t, err)

	assert.Len(t, server.Services.Instances, 1, "preview must not delete")
	assert.False(t, server.HasDeleteRequest(), "no DELETE call should be made in preview")
	assert.Contains(t, result.Stdout, "would delete")
	assert.Contains(t, result.Stdout, "--confirm")
	assert.Contains(t, result.Stdout, svc.Id)
	assert.Contains(t, result.Stdout, "my-api")
}

func TestServiceDelete_ConfirmByName_Deletes(t *testing.T) {
	server := renderapi.NewServer(t)
	svc := seedService(server, "by-name-api")

	result, err := executeServiceDelete(t, server, "by-name-api", "--confirm", "--output", "text")
	require.NoError(t, err)

	assert.Empty(t, server.Services.Instances)
	assert.True(t, server.HasRequest("DELETE", "/services/"+svc.Id))
	assert.Contains(t, result.Stdout, "Deleted")
	assert.Contains(t, result.Stdout, svc.Id)
}

func TestServiceDelete_DatastoreIDsAreNotServices(t *testing.T) {
	server := renderapi.NewServer(t)
	pg := server.Postgres.Add(renderapi.NewPostgres(client.PostgresDetail{
		Name:  "app-db",
		Owner: client.Owner{Id: serviceTestWorkspaceID},
	}))

	_, err := executeServiceDelete(t, server, pg.Id, "--confirm", "--output", "text")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "No service named")
	assert.Contains(t, err.Error(), serviceTestWorkspaceName)
	assert.Len(t, server.Postgres.Instances, 1)
	assert.False(t, server.HasDeleteRequest())
}

func TestServiceDelete_NameCollision_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	seedService(server, "not-unique")
	seedService(server, "not-unique")

	_, err := executeServiceDelete(t, server, "not-unique", "--confirm", "--output", "text")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "Multiple services found")
	assert.Len(t, server.Services.Instances, 2, "no delete on ambiguity")
	assert.False(t, server.HasDeleteRequest())
}

func TestServiceDelete_JSONOutput_AfterConfirm(t *testing.T) {
	server := renderapi.NewServer(t)
	svc := seedService(server, "json-api")

	result, err := executeServiceDelete(t, server, svc.Id, "--confirm", "--output", "json")
	require.NoError(t, err)
	assert.Empty(t, server.Services.Instances)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	require.Len(t, body, 2)

	data := requireSubMap(t, body, "data")
	meta := requireSubMap(t, body, "meta")

	assert.Equal(t, svc.Id, data["id"])
	assert.Equal(t, "json-api", data["name"])
	assert.Equal(t, serviceTestWorkspaceID, data["ownerId"])
	assert.Equal(t, string(client.WebService), data["type"])
	assert.Contains(t, data, "serviceDetails")
	assert.Equal(t, true, meta["deleted"])
}

func TestServiceDelete_JSONOutput_PreviewIncludesConfirmMessage(t *testing.T) {
	server := renderapi.NewServer(t)
	svc := seedService(server, "json-api")

	result, err := executeServiceDelete(t, server, svc.Id, "--output", "json")
	require.NoError(t, err)
	assert.Len(t, server.Services.Instances, 1, "preview must not delete")

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))

	data := requireSubMap(t, body, "data")
	meta := requireSubMap(t, body, "meta")
	assert.Equal(t, svc.Id, data["id"])
	assert.Equal(t, false, meta["deleted"])
	assert.Equal(t, "re-run with --confirm to delete", meta["message"])
}

func TestServiceDelete_RequiresActiveWorkspace(t *testing.T) {
	server := renderapi.NewServer(t)
	svc := seedService(server, "my-api")

	_, err := executeServiceDeleteWithoutActiveWorkspace(t, server, svc.Id, "--confirm", "--output", "text")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "workspace")
	assert.Len(t, server.Services.Instances, 1)
	assert.False(t, server.HasDeleteRequest())
}

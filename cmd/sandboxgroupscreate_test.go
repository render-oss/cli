package cmd

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
)

type sandboxGroupsCreateHarness struct {
	t      *testing.T
	server *renderapi.Server
}

func newSandboxGroupsCreateHarness(t *testing.T) sandboxGroupsCreateHarness {
	t.Helper()

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: sandboxGroupsActiveWorkspaceID, Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", sandboxGroupsActiveWorkspaceID)

	return sandboxGroupsCreateHarness{t: t, server: server}
}

func (h sandboxGroupsCreateHarness) execute(extraArgs ...string) (CommandResult, error) {
	h.t.Helper()
	return executeSandboxGroupsCommand(h.t, h.server, append([]string{"ea", "sandbox-groups", "create"}, extraArgs...)...)
}

func unmarshalSandboxGroupJSONOutput(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &data), "expected a JSON object, got: %s", stdout)
	return data
}

func TestSandboxGroupsCreate_SendsNameAndWorkspace(t *testing.T) {
	h := newSandboxGroupsCreateHarness(t)

	result, err := h.execute("--name", "agents", "--output", "json")
	require.NoError(t, err)

	req, ok := h.server.LastRequest("POST", "/sandbox-groups")
	require.True(t, ok)
	body := testrequire.ParseJSONMap(t, string(req.Body))
	assert.Equal(t, sandboxGroupsActiveWorkspaceID, body["ownerId"])
	assert.Equal(t, "agents", body["name"])
	assert.NotContains(t, body, "region")

	require.Len(t, h.server.SandboxGroups.Instances, 1)
	created := h.server.SandboxGroups.Instances[0]
	data := unmarshalSandboxGroupJSONOutput(t, result.Stdout)
	assert.Equal(t, created.Id, data["id"])
	assert.Equal(t, "agents", data["name"])
	assert.Equal(t, "oregon", data["region"])
	assert.Equal(t, true, data["isDefault"])
}

func TestSandboxGroupsCreate_TextOutput(t *testing.T) {
	h := newSandboxGroupsCreateHarness(t)

	result, err := h.execute("--name", "agents", "--output", "text")
	require.NoError(t, err)

	require.Len(t, h.server.SandboxGroups.Instances, 1)
	created := h.server.SandboxGroups.Instances[0]
	assert.Contains(t, result.Stdout, created.Id)
	assert.Contains(t, result.Stdout, "Name:              agents")
	assert.Contains(t, result.Stdout, "Region:            oregon")
	assert.Contains(t, result.Stdout, "Default:           true")
}

func TestSandboxGroupsCreate_YAMLOutput(t *testing.T) {
	h := newSandboxGroupsCreateHarness(t)

	result, err := h.execute("--name", "agents", "--output", "yaml")
	require.NoError(t, err)

	require.Len(t, h.server.SandboxGroups.Instances, 1)
	created := h.server.SandboxGroups.Instances[0]
	assert.Contains(t, result.Stdout, created.Id)
	assert.Contains(t, result.Stdout, "name: agents")
	assert.Contains(t, result.Stdout, "region: oregon")
}

func TestSandboxGroupsCreate_DefaultOutput_TreatedAsText(t *testing.T) {
	h := newSandboxGroupsCreateHarness(t)

	result, err := h.execute("--name", "agents")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "Name:              agents")
}

func TestSandboxGroupsCreate_SecondGroupIsNotDefault(t *testing.T) {
	h := newSandboxGroupsCreateHarness(t)
	seedSandboxGroup(h.server, "Default")

	result, err := h.execute("--name", "agents", "--output", "json")
	require.NoError(t, err)

	data := unmarshalSandboxGroupJSONOutput(t, result.Stdout)
	assert.Equal(t, "agents", data["name"])
	assert.Equal(t, false, data["isDefault"])
	assert.Len(t, h.server.SandboxGroups.Instances, 2)
}

func TestSandboxGroupsCreate_RequiresName(t *testing.T) {
	h := newSandboxGroupsCreateHarness(t)

	_, err := h.execute("--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--name is required")

	_, ok := h.server.LastRequest("POST", "/sandbox-groups")
	assert.False(t, ok, "no request should be sent without a name")
}

func TestSandboxGroupsCreate_RejectsPositionalArgs(t *testing.T) {
	h := newSandboxGroupsCreateHarness(t)

	result, err := h.execute("agents")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
	assert.NotContains(t, result.Stderr, "Usage:")
}

func TestSandboxGroupsCreate_NameTaken_Surfaces409(t *testing.T) {
	h := newSandboxGroupsCreateHarness(t)
	seedSandboxGroup(h.server, "agents")

	_, err := h.execute("--name", "agents", "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sandbox group name is taken")
	assert.Len(t, h.server.SandboxGroups.Instances, 1)
}

func TestSandboxGroupsCreate_APIError_Surfaces(t *testing.T) {
	h := newSandboxGroupsCreateHarness(t)
	h.server.SandboxGroups.RespondWith(http.StatusInternalServerError)

	_, err := h.execute("--name", "agents", "--output", "json")
	require.Error(t, err)
	assert.Empty(t, h.server.SandboxGroups.Instances)
}

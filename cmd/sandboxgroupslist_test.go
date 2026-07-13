package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
)

type sandboxGroupsListHarness struct {
	t      *testing.T
	server *renderapi.Server
}

func newSandboxGroupsListHarness(t *testing.T) sandboxGroupsListHarness {
	t.Helper()

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: sandboxGroupsActiveWorkspaceID, Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", sandboxGroupsActiveWorkspaceID)

	return sandboxGroupsListHarness{t: t, server: server}
}

func (h sandboxGroupsListHarness) execute(extraArgs ...string) (CommandResult, error) {
	h.t.Helper()
	return executeSandboxGroupsCommand(h.t, h.server, append([]string{"ea", "sandbox-groups", "list"}, extraArgs...)...)
}

func TestSandboxGroupsList_NoGroups(t *testing.T) {
	h := newSandboxGroupsListHarness(t)

	// text
	result, err := h.execute("--output", "text")
	require.NoError(t, err)
	assert.NotContains(t, result.Stdout, "sbg-")

	// json expects an empty array, not an error
	result, err = h.execute("--output", "json")
	require.NoError(t, err)
	body := unmarshalSandboxGroupsJSONOutput(t, result.Stdout)
	assert.Empty(t, body)
}

func TestSandboxGroupsList_SingleGroup(t *testing.T) {
	h := newSandboxGroupsListHarness(t)
	group := seedSandboxGroup(h.server, "Default")

	result, err := h.execute("--output", "text")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, group.Id)
	assert.Contains(t, result.Stdout, "Default")
	assert.Contains(t, result.Stdout, "oregon")
}

func TestSandboxGroupsList_JSONOutput(t *testing.T) {
	h := newSandboxGroupsListHarness(t)
	group := seedSandboxGroup(h.server, "Default")

	result, err := h.execute("--output", "json")
	require.NoError(t, err)

	data := unmarshalSandboxGroupsJSONOutput(t, result.Stdout)
	require.Len(t, data, 1)
	first, ok := data[0].(map[string]any)
	require.True(t, ok, "expected object, got %#v", data[0])
	assert.Equal(t, group.Id, first["id"])
	assert.Equal(t, "Default", first["name"])
	assert.Equal(t, "oregon", first["region"])
	assert.Equal(t, true, first["isDefault"])
}

func TestSandboxGroupsList_YAMLOutput(t *testing.T) {
	h := newSandboxGroupsListHarness(t)
	group := seedSandboxGroup(h.server, "Default")

	result, err := h.execute("--output", "yaml")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, group.Id)
	assert.Contains(t, result.Stdout, "name: Default")
	assert.Contains(t, result.Stdout, "region: oregon")
}

func TestSandboxGroupsList_DefaultOutput_TreatedAsText(t *testing.T) {
	h := newSandboxGroupsListHarness(t)
	group := seedSandboxGroup(h.server, "Default")

	result, err := h.execute()
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, group.Id)
	assert.Contains(t, result.Stdout, "Default")
}

func TestSandboxGroupsList_OnlyReturnsActiveWorkspaceGroups(t *testing.T) {
	h := newSandboxGroupsListHarness(t)
	// Seed a group for the active workspace and a decoy for another workspace
	// (both owners must be seeded so the fake accepts them).
	h.server.Owners.Add(renderapi.NewOwner(client.Owner{Id: "tea-someone-else-abc"}))
	mine := seedSandboxGroup(h.server, "Mine")
	other := seedSandboxGroupForOwner(h.server, "Not mine", "tea-someone-else-abc")

	result, err := h.execute("--output", "json")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, mine.Id)
	assert.NotContains(t, result.Stdout, other.Id)
}

func TestSandboxGroupsList_RejectsExtraArgs(t *testing.T) {
	h := newSandboxGroupsListHarness(t)

	result, err := h.execute("junk")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
	// SilenceUsage keeps arg errors terse, matching pg and kv list.
	assert.NotContains(t, result.Stderr, "Usage:")
}

func TestSandboxGroupsList_APIError_Surfaces(t *testing.T) {
	h := newSandboxGroupsListHarness(t)
	h.server.SandboxGroups.RespondWith(500)

	_, err := h.execute("--output", "text")
	require.Error(t, err)
}

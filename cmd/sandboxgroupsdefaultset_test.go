package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

type sandboxGroupsDefaultSetHarness struct {
	t            *testing.T
	server       *renderapi.Server
	defaultGroup *sandboxesclient.SandboxGroup
	otherGroup   *sandboxesclient.SandboxGroup
}

func newSandboxGroupsDefaultSetHarness(t *testing.T) sandboxGroupsDefaultSetHarness {
	t.Helper()
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: sandboxGroupsActiveWorkspaceID}))
	t.Setenv("RENDER_WORKSPACE", sandboxGroupsActiveWorkspaceID)

	defaultGroup := seedSandboxGroup(server, "Default")
	otherGroup := seedSandboxGroup(server, "agents")
	otherGroup.IsDefault = false
	return sandboxGroupsDefaultSetHarness{t: t, server: server, defaultGroup: defaultGroup, otherGroup: otherGroup}
}

func (h sandboxGroupsDefaultSetHarness) execute(args ...string) (CommandResult, error) {
	h.t.Helper()
	args = append([]string{"sandbox-groups", "default", "set"}, args...)
	return executeSandboxGroupsCommand(h.t, h.server, args...)
}

func TestSandboxGroupsDefaultSet_MovesDefault(t *testing.T) {
	h := newSandboxGroupsDefaultSetHarness(t)

	result, err := h.execute(h.otherGroup.Id, "--output", "json")
	require.NoError(t, err)

	assert.True(t, h.server.HasRequest("POST", "/sandbox-groups/"+h.otherGroup.Id+"/set-default"))
	data := unmarshalSandboxGroupJSONOutput(t, result.Stdout)
	assert.Equal(t, h.otherGroup.Id, data["id"])
	assert.Equal(t, true, data["isDefault"])
	assert.True(t, h.otherGroup.IsDefault)
	assert.False(t, h.defaultGroup.IsDefault)
}

func TestSandboxGroupsDefaultSet_Output(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
	}{
		{name: "text", args: []string{"--output", "text"}, wantStdout: "Default:           true"},
		{name: "default format is text", wantStdout: "Default:           true"},
		{name: "yaml", args: []string{"--output", "yaml"}, wantStdout: "isDefault: true"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxGroupsDefaultSetHarness(t)

			result, err := h.execute(append([]string{h.otherGroup.Id}, tc.args...)...)
			require.NoError(t, err)

			assert.Contains(t, result.Stdout, h.otherGroup.Id)
			assert.Contains(t, result.Stdout, tc.wantStdout)
		})
	}
}

func TestSandboxGroupsDefaultSet_AlreadyDefaultIsIdempotent(t *testing.T) {
	h := newSandboxGroupsDefaultSetHarness(t)

	result, err := h.execute(h.defaultGroup.Id, "--output", "json")
	require.NoError(t, err)

	data := unmarshalSandboxGroupJSONOutput(t, result.Stdout)
	assert.Equal(t, true, data["isDefault"])
	assert.True(t, h.defaultGroup.IsDefault)
	assert.False(t, h.otherGroup.IsDefault)
}

func TestSandboxGroupsDefaultSet_Errors(t *testing.T) {
	tests := []struct {
		name      string
		groupID   string
		suspend   bool
		wantError string
	}{
		{name: "unknown group", groupID: "sbg-doesnotexist000000", wantError: "404"},
		{
			name:      "suspended group",
			suspend:   true,
			wantError: "the sandbox group is suspended and cannot be made the default",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxGroupsDefaultSetHarness(t)
			groupID := tc.groupID
			if tc.suspend {
				groupID = h.otherGroup.Id
				h.server.SandboxGroups.Suspend(groupID)
			}

			_, err := h.execute(groupID, "--output", "json")
			require.Error(t, err)

			assert.Contains(t, err.Error(), tc.wantError)
			assert.True(t, h.defaultGroup.IsDefault)
			assert.False(t, h.otherGroup.IsDefault)
		})
	}
}

func TestSandboxGroupsDefaultSet_RequiresGroupID(t *testing.T) {
	h := newSandboxGroupsDefaultSetHarness(t)

	_, err := h.execute()
	require.Error(t, err)

	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestSandboxGroupsDefault_WithoutSubcommandShowsHelp(t *testing.T) {
	h := newSandboxGroupsDefaultSetHarness(t)

	result, err := executeSandboxGroupsCommand(t, h.server, "sandbox-groups", "default")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "set")
	assert.Empty(t, h.server.Requests)
}

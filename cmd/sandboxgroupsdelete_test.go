package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/sandboxgroup"
)

type sandboxGroupsDeleteHarness struct {
	t            *testing.T
	server       *renderapi.Server
	defaultGroup *sandboxesclient.SandboxGroup
	otherGroup   *sandboxesclient.SandboxGroup
}

func newSandboxGroupsDeleteHarness(t *testing.T) sandboxGroupsDeleteHarness {
	t.Helper()
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: sandboxGroupsActiveWorkspaceID}))
	t.Setenv("RENDER_WORKSPACE", sandboxGroupsActiveWorkspaceID)

	defaultGroup := seedSandboxGroup(server, "Default")
	otherGroup := seedSandboxGroup(server, "agents")
	otherGroup.IsDefault = false
	otherGroup.Region = "frankfurt"
	return sandboxGroupsDeleteHarness{t: t, server: server, defaultGroup: defaultGroup, otherGroup: otherGroup}
}

func (h sandboxGroupsDeleteHarness) execute(args ...string) (CommandResult, error) {
	h.t.Helper()
	return executeSandboxGroupsCommand(h.t, h.server, append([]string{"sandbox-groups", "delete"}, args...)...)
}

func (h sandboxGroupsDeleteHarness) groupIDs() []string {
	ids := make([]string, 0, len(h.server.SandboxGroups.Instances))
	for _, g := range h.server.SandboxGroups.Instances {
		ids = append(ids, g.Id)
	}
	return ids
}

func TestSandboxGroupsDelete_PreviewShowsGroupAndKeepsIt(t *testing.T) {
	h := newSandboxGroupsDeleteHarness(t)

	result, err := h.execute(h.otherGroup.Id, "--output", "text")
	require.NoError(t, err)

	for _, want := range []string{
		"This command would delete this sandbox group",
		"ID:                " + h.otherGroup.Id,
		"Name:              agents",
		"Region:            frankfurt",
		"terminates its sandboxes and deletes its snapshots and credentials",
		"Re-run with --confirm to proceed",
	} {
		assert.Contains(t, result.Stdout, want)
	}
	assert.True(t, h.server.HasRequest("GET", "/sandbox-groups/"+h.otherGroup.Id))
	assert.False(t, h.server.HasDeleteRequest())
	assert.Contains(t, h.groupIDs(), h.otherGroup.Id)
}

func TestSandboxGroupsDelete_ConfirmDeletesGroup(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantStdout string
	}{
		{name: "text", output: "text", wantStdout: "Deleted sandbox group "},
		{name: "json", output: "json", wantStdout: `"deleted": true`},
		{name: "yaml", output: "yaml", wantStdout: "deleted: true"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxGroupsDeleteHarness(t)

			result, err := h.execute(h.otherGroup.Id, "--confirm", "--output", tc.output)
			require.NoError(t, err)

			assert.True(t, h.server.HasRequest("DELETE", "/sandbox-groups/"+h.otherGroup.Id))
			assert.Equal(t, []string{h.defaultGroup.Id}, h.groupIDs())
			assert.Contains(t, result.Stdout, tc.wantStdout)
			assert.Contains(t, result.Stdout, h.otherGroup.Id)
		})
	}
}

func TestSandboxGroupsDelete_JSONOutputIncludesGroup(t *testing.T) {
	tests := []struct {
		name        string
		confirm     bool
		wantDeleted bool
		wantMessage string
	}{
		{name: "preview", wantMessage: "re-run with --confirm to delete"},
		{name: "confirmed", confirm: true, wantDeleted: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxGroupsDeleteHarness(t)
			args := []string{h.otherGroup.Id, "--output", "json"}
			if tc.confirm {
				args = append(args, "--confirm")
			}

			result, err := h.execute(args...)
			require.NoError(t, err)

			var out sandboxgroup.DeleteOut
			require.NoError(t, json.Unmarshal([]byte(result.Stdout), &out))
			require.NotNil(t, out.Data)
			assert.Equal(t, h.otherGroup.Id, out.Data.Id)
			assert.Equal(t, "agents", out.Data.Name)
			assert.Equal(t, "frankfurt", out.Data.Region)
			assert.Equal(t, tc.wantDeleted, out.Meta.Deleted)
			assert.Equal(t, tc.wantMessage, out.Meta.Message)
		})
	}
}

const defaultGroupNotDeletableError = "default sandbox group cannot be deleted; make another group the default with " +
	"`render sandbox-groups default set <sandboxGroupId>` first"

func TestSandboxGroupsDelete_RejectedBeforeAnyDelete(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		confirm   bool
		wantError string
	}{
		{name: "unknown group preview", target: "unknown", wantError: "404"},
		{name: "unknown group confirmed", target: "unknown", confirm: true, wantError: "404"},
		{
			name:      "default group preview",
			target:    "default",
			wantError: defaultGroupNotDeletableError,
		},
		{
			name:      "default group confirmed",
			target:    "default",
			confirm:   true,
			wantError: defaultGroupNotDeletableError,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxGroupsDeleteHarness(t)
			targetID := "sbg-doesnotexist000000"
			if tc.target == "default" {
				targetID = h.defaultGroup.Id
			}
			args := []string{targetID, "--output", "text"}
			if tc.confirm {
				args = append(args, "--confirm")
			}

			result, err := h.execute(args...)
			require.Error(t, err)

			assert.Contains(t, err.Error(), tc.wantError)
			assert.NotContains(t, result.Stdout, "would delete")
			assert.False(t, h.server.HasDeleteRequest())
			assert.ElementsMatch(t, []string{h.defaultGroup.Id, h.otherGroup.Id}, h.groupIDs())
		})
	}
}

func TestSandboxGroupsDelete_SuspendedGroupExplainsWhy(t *testing.T) {
	h := newSandboxGroupsDeleteHarness(t)
	h.server.SandboxGroups.Suspend(h.otherGroup.Id)

	_, err := h.execute(h.otherGroup.Id, "--confirm", "--output", "json")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "sandbox group is suspended and cannot be deleted")
	assert.NotContains(t, err.Error(), "received response code")
	assert.ElementsMatch(t, []string{h.defaultGroup.Id, h.otherGroup.Id}, h.groupIDs())
}

func TestSandboxGroupsDelete_RequiresGroupID(t *testing.T) {
	h := newSandboxGroupsDeleteHarness(t)

	_, err := h.execute("--confirm")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "accepts 1 arg")
	assert.False(t, h.server.HasDeleteRequest())
}

package cmd

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	sandboxes "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/sandbox"
)

func TestSandboxList_StatusHelpListsEveryStatus(t *testing.T) {
	server := renderapi.NewServer(t)

	result, err := executeSandboxCommand(t, server, "ea", "sandboxes", "list", "--help")
	require.NoError(t, err)

	for _, status := range sandbox.AllStatuses() {
		assert.Contains(t, result.Stdout, string(status), "--status help should list %q", status)
	}
}

func TestSandboxList_GroupSelection(t *testing.T) {
	server := newSandboxCreateServer(t)
	groupID := "sbg-cph1rs3idesc73a2b2mg"
	server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxes.SandboxGroup{Id: groupID, OwnerId: sandboxCreateActiveWorkspaceID}))
	server.Sandboxes.Add(renderapi.NewSandbox(sandboxes.Sandbox{Id: "sbx-selected", SandboxGroupId: &groupID}))
	server.Sandboxes.Add(renderapi.NewSandbox(sandboxes.Sandbox{Id: "sbx-other", SandboxGroupId: new(sandboxCreateDefaultGroup(t, server).Id)}))
	result, err := executeSandboxCommand(t, server, "ea", "sandboxes", "list", "--sandbox-group", groupID, "--output", "json")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "sbx-selected")
	assert.NotContains(t, result.Stdout, "sbx-other")
	req, ok := server.LastRequest("GET", "/sandboxes")
	require.True(t, ok)
	u, err := url.Parse(req.URI)
	require.NoError(t, err)
	assert.Equal(t, groupID, u.Query().Get("sandboxGroupId"))
}

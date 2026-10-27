package cmd

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testrequire"
	sandboxes "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/sandbox"
)

func TestSandboxList_StatusHelpListsEveryStatus(t *testing.T) {
	server := renderapi.NewServer(t)

	result, err := executeSandboxCommand(t, server, "sandboxes", "list", "--help")
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
	result, err := executeSandboxCommand(t, server, "sandboxes", "list", "--group", groupID, "--output", "json")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "sbx-selected")
	assert.NotContains(t, result.Stdout, "sbx-other")
	req, ok := server.LastRequest("GET", "/sandboxes")
	require.True(t, ok)
	u, err := url.Parse(req.URI)
	require.NoError(t, err)
	assert.Equal(t, groupID, u.Query().Get("sandboxGroupId"))
}

func TestSandboxGroupFlagAlias(t *testing.T) {
	groupID := "sbg-cph1rs3idesc73a2b2mg"
	for _, command := range []string{"create", "list"} {
		t.Run(command, func(t *testing.T) {
			server := newSandboxCreateServer(t)
			server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxes.SandboxGroup{Id: groupID, OwnerId: sandboxCreateActiveWorkspaceID}))

			help, err := executeSandboxCommand(t, server, "sandboxes", command, "--help")
			require.NoError(t, err)
			assert.NotContains(t, help.Stdout, "--sandbox-group", "the alias is hidden from help")

			_, err = executeSandboxCommand(t, server, "sandboxes", command, "--sandbox-group", groupID, "--output", "json")
			require.NoError(t, err)
			switch command {
			case "create":
				req, ok := server.LastRequest("POST", "/sandboxes")
				require.True(t, ok)
				assert.Equal(t, groupID, testrequire.ParseJSONMap(t, string(req.Body))["sandboxGroupId"])
			case "list":
				req, ok := server.LastRequest("GET", "/sandboxes")
				require.True(t, ok)
				u, err := url.Parse(req.URI)
				require.NoError(t, err)
				assert.Equal(t, groupID, u.Query().Get("sandboxGroupId"))
			}
		})
	}
}

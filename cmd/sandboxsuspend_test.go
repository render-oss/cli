package cmd

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	sandboxes "github.com/render-oss/cli/pkg/client/sandboxes"
)

func TestSandboxSuspend_SuspendsRunningSandbox(t *testing.T) {
	server := newSandboxCreateServer(t)
	server.Sandboxes.Add(renderapi.NewSandbox(sandboxes.Sandbox{Id: "sbx-run", Status: sandboxes.SandboxStatusRunning}))

	result, err := executeSandboxCommand(t, server, "sandboxes", "suspend", "sbx-run", "--output", "text")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "ID:      sbx-run")
	assert.Contains(t, result.Stdout, "Status:  suspended")

	req, ok := server.LastRequest("POST", "/sandboxes/sbx-run/suspend")
	require.True(t, ok)
	u, err := url.Parse(req.URI)
	require.NoError(t, err)
	assert.Equal(t, sandboxCreateActiveWorkspaceID, u.Query().Get("ownerId"))
}

func TestSandboxSuspend_WaitJSON(t *testing.T) {
	server := newSandboxCreateServer(t)
	server.Sandboxes.Add(renderapi.NewSandbox(sandboxes.Sandbox{Id: "sbx-run", Status: sandboxes.SandboxStatusRunning}))

	result, err := executeSandboxCommand(t, server, "sandboxes", "suspend", "sbx-run", "--wait", "--output", "json")
	require.NoError(t, err)

	var got sandboxes.Sandbox
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &got), "stdout should be a single JSON sandbox: %s", result.Stdout)
	assert.Equal(t, sandboxes.SandboxStatusSuspended, got.Status)
}

func TestSandboxSuspend_AlreadySuspendedSucceeds(t *testing.T) {
	server := newSandboxCreateServer(t)
	server.Sandboxes.Add(renderapi.NewSandbox(sandboxes.Sandbox{Id: "sbx-sus", Status: sandboxes.SandboxStatusSuspended}))

	result, err := executeSandboxCommand(t, server, "sandboxes", "suspend", "sbx-sus", "--output", "text")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "Status:  suspended")
}

func TestSandboxSuspend_NotRunningFails(t *testing.T) {
	server := newSandboxCreateServer(t)
	server.Sandboxes.Add(renderapi.NewSandbox(sandboxes.Sandbox{Id: "sbx-new", Status: sandboxes.SandboxStatusCreating}))

	result, err := executeSandboxCommand(t, server, "sandboxes", "suspend", "sbx-new", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "409")
	assert.Contains(t, err.Error(), "sandbox is not running")
	assert.Empty(t, result.Stdout)
}

func TestSandboxSuspend_NotFound(t *testing.T) {
	server := newSandboxCreateServer(t)

	_, err := executeSandboxCommand(t, server, "sandboxes", "suspend", "sbx-missing", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

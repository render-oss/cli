package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

var sandboxSnapshotsActiveWorkspaceID = testids.WorkspaceID("snapshots")

type sandboxSnapshotsHarness struct {
	t       *testing.T
	server  *renderapi.Server
	group   *sandboxesclient.SandboxGroup
	sandbox *sandboxesclient.Sandbox
}

func newSandboxSnapshotsHarness(t *testing.T) sandboxSnapshotsHarness {
	t.Helper()

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: sandboxSnapshotsActiveWorkspaceID, Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", sandboxSnapshotsActiveWorkspaceID)
	group := server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{OwnerId: sandboxSnapshotsActiveWorkspaceID}))
	sandbox := server.Sandboxes.Add(renderapi.NewSandbox(sandboxesclient.Sandbox{}))

	return sandboxSnapshotsHarness{t: t, server: server, group: group, sandbox: sandbox}
}

func (h sandboxSnapshotsHarness) execute(args ...string) (CommandResult, error) {
	h.t.Helper()
	return executeSandboxCommand(h.t, h.server, append([]string{"ea", "sandboxes", "snapshots"}, args...)...)
}

func TestSandboxSnapshots_HelpListsSubcommands(t *testing.T) {
	h := newSandboxSnapshotsHarness(t)

	result, err := h.execute("--help")
	require.NoError(t, err)
	for _, sub := range []string{"create"} {
		assert.Contains(t, result.Stdout, sub, "expected subcommand %q in help output", sub)
	}
}

func TestSandboxSnapshotsCreate_TextOutput(t *testing.T) {
	h := newSandboxSnapshotsHarness(t)

	result, err := h.execute("create", h.sandbox.Id, "--output", "text")
	require.NoError(t, err)

	created := h.server.SandboxSnapshots.Only(t)
	assert.Contains(t, result.Stdout, created.Id)
	assert.Contains(t, result.Stdout, "creating")
	assert.Contains(t, result.Stdout, "filesystem")
	assert.Contains(t, result.Stdout, h.sandbox.Id)
	assert.False(t, h.server.HasRequest("GET", "/snapshots"), "create must return without polling")
}

func TestSandboxSnapshotsCreate_JSONOutput(t *testing.T) {
	h := newSandboxSnapshotsHarness(t)

	result, err := h.execute("create", h.sandbox.Id, "--output", "json")
	require.NoError(t, err)

	created := h.server.SandboxSnapshots.Only(t)
	body := testrequire.ParseJSONMap(t, result.Stdout)
	assert.Equal(t, created.Id, body["id"])
	assert.Equal(t, "creating", body["status"])
	assert.Equal(t, "filesystem", body["kind"])
	assert.Equal(t, h.sandbox.Id, body["sourceSandboxId"])
	assert.Equal(t, h.group.Id, body["sandboxGroupId"])
}

func TestSandboxSnapshotsCreate_Kind(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantBody    string
		wantAbsent  bool
		errContains string
	}{
		{name: "default omits kind so the API picks filesystem", wantAbsent: true},
		{name: "filesystem", args: []string{"--kind", "filesystem"}, wantBody: "filesystem"},
		{name: "runtime", args: []string{"--kind", "runtime"}, wantBody: "runtime"},
		{name: "unknown kind is rejected before any request", args: []string{"--kind", "memory"}, errContains: `invalid kind "memory"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxSnapshotsHarness(t)

			_, err := h.execute(append([]string{"create", h.sandbox.Id, "--output", "json"}, tc.args...)...)
			if tc.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				assert.Contains(t, err.Error(), "filesystem")
				assert.Contains(t, err.Error(), "runtime")
				assert.False(t, h.server.HasRequest("POST", "/snapshots"))
				return
			}
			require.NoError(t, err)

			req, ok := h.server.LastRequest("POST", "/sandboxes/"+h.sandbox.Id+"/snapshots")
			require.True(t, ok, "expected a create request")
			assert.Contains(t, req.URI, "ownerId="+sandboxSnapshotsActiveWorkspaceID)
			body := testrequire.ParseJSONMap(t, string(req.Body))
			kind, present := body["kind"]
			if tc.wantAbsent {
				assert.False(t, present, "kind should be omitted, got %v", kind)
				return
			}
			assert.Equal(t, tc.wantBody, kind)
		})
	}
}

func TestSandboxSnapshotsCreate_RequiresSandboxID(t *testing.T) {
	h := newSandboxSnapshotsHarness(t)

	_, err := h.execute("create")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestSandboxSnapshotsCreate_SandboxNotFound(t *testing.T) {
	h := newSandboxSnapshotsHarness(t)

	_, err := h.execute("create", testids.SandboxID("missing"), "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "sandbox not found")
}

func TestSandboxSnapshotsCreate_SandboxNotRunning_SurfacesCode(t *testing.T) {
	h := newSandboxSnapshotsHarness(t)
	suspended := h.server.Sandboxes.Add(renderapi.NewSandbox(sandboxesclient.Sandbox{Status: sandboxesclient.SandboxStatusSuspended}))

	_, err := h.execute("create", suspended.Id, "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "409 (sandbox_not_running)")
	assert.Empty(t, h.server.SandboxSnapshots.Instances)
}

package cmd

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

var sandboxBuildsActiveWorkspaceID = testids.WorkspaceID("builds")

type sandboxBuildsHarness struct {
	t      *testing.T
	server *renderapi.Server
	group  *sandboxesclient.SandboxGroup
}

func newSandboxBuildsHarness(t *testing.T) sandboxBuildsHarness {
	t.Helper()

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: sandboxBuildsActiveWorkspaceID, Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", sandboxBuildsActiveWorkspaceID)
	group := server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{OwnerId: sandboxBuildsActiveWorkspaceID}))

	return sandboxBuildsHarness{t: t, server: server, group: group}
}

func (h sandboxBuildsHarness) execute(args ...string) (CommandResult, error) {
	h.t.Helper()
	return h.executeWithStdin("", args...)
}

func (h sandboxBuildsHarness) executeWithStdin(stdin string, args ...string) (CommandResult, error) {
	h.t.Helper()
	return executeSandboxGroupsCommandWithStdin(h.t, h.server, stdin, append([]string{"sandbox-groups", "builds"}, args...)...)
}

func (h sandboxBuildsHarness) seedBuild(b sandboxesclient.SandboxBuild) *sandboxesclient.SandboxBuild {
	if b.SandboxGroupId == "" {
		b.SandboxGroupId = h.group.Id
	}
	return h.server.SandboxBuilds.Add(renderapi.NewSandboxBuild(b))
}

func TestSandboxGroups_HelpListsBuilds(t *testing.T) {
	h := newSandboxBuildsHarness(t)

	result, err := executeSandboxGroupsCommand(t, h.server, "sandbox-groups", "--help")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "builds")
}

func TestSandboxBuilds_HelpListsSubcommands(t *testing.T) {
	h := newSandboxBuildsHarness(t)

	result, err := h.execute("--help")
	require.NoError(t, err)
	for _, sub := range []string{"create", "get"} {
		assert.Contains(t, result.Stdout, sub, "expected subcommand %q in help output", sub)
	}
}

func TestSandboxBuildsCreate_TextOutput(t *testing.T) {
	h := newSandboxBuildsHarness(t)

	result, err := h.execute("create", "python:3.13", "--name", "python", "--output", "text")
	require.NoError(t, err)

	created := h.server.SandboxBuilds.Only(t)
	for _, want := range []string{created.Id, "Name:           python", "running", "docker.io/library/python:3.13", h.group.Id, "Finished:       -"} {
		assert.Contains(t, result.Stdout, want)
	}
	assert.NotContains(t, result.Stdout, "Failure reason")
	assert.False(t, h.server.HasRequest("GET", "/builds"), "create must return without polling")
}

func TestSandboxBuildsCreate_JSONOutput(t *testing.T) {
	h := newSandboxBuildsHarness(t)

	result, err := h.execute("create", "python:3.13", "--name", "python", "--output", "json")
	require.NoError(t, err)

	created := h.server.SandboxBuilds.Only(t)
	body := testrequire.ParseJSONMap(t, result.Stdout)
	assert.Equal(t, created.Id, body["id"])
	assert.Equal(t, "running", body["status"])
	assert.Equal(t, "python", body["name"])
	assert.Equal(t, h.group.Id, body["sandboxGroupId"])
	assert.Equal(t, map[string]any{"repository": "docker.io/library", "image": "python", "reference": "3.13"}, body["source"])

	req, ok := h.server.LastRequest("POST", "/sandbox-groups/"+h.group.Id+"/builds")
	require.True(t, ok, "expected a create request")
	assert.NotContains(t, req.URI, "ownerId")
	reqBody := testrequire.ParseJSONMap(t, string(req.Body))
	assert.Equal(t, "python", reqBody["name"])
	assert.Equal(t, "python:3.13", reqBody["sourceImage"])
	assert.NotContains(t, reqBody, "registryCredential")
}

func TestSandboxBuildsCreate_RegistryCredential(t *testing.T) {
	h := newSandboxBuildsHarness(t)

	_, err := h.executeWithStdin("s3cret\n", "create", "ghcr.io/acme/agent:1.0", "--name", "agent",
		"--registry-username", "bot", "--registry-password-stdin", "--output", "json")
	require.NoError(t, err)

	req, ok := h.server.LastRequest("POST", "/sandbox-groups/"+h.group.Id+"/builds")
	require.True(t, ok, "expected a create request")
	body := testrequire.ParseJSONMap(t, string(req.Body))
	assert.Equal(t, map[string]any{"username": "bot", "password": "s3cret"}, body["registryCredential"])
}

func TestSandboxBuildsCreate_RegistryFlagsRequiredTogether(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
	}{
		{name: "username alone", args: []string{"--registry-username", "bot"}},
		{name: "stdin alone", args: []string{"--registry-password-stdin"}, stdin: "s3cret\n"},
		{name: "empty username", args: []string{"--registry-username", "", "--registry-password-stdin"}, stdin: "s3cret\n"},
		{name: "empty stdin", args: []string{"--registry-username", "bot", "--registry-password-stdin"}, stdin: "\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxBuildsHarness(t)

			_, err := h.executeWithStdin(tc.stdin, append([]string{"create", "python:3.13", "--name", "python", "--output", "json"}, tc.args...)...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "registry-password-stdin")
			assert.False(t, h.server.HasRequest("POST", "/builds"))
		})
	}
}

func TestSandboxBuildsCreate_RequiresName(t *testing.T) {
	h := newSandboxBuildsHarness(t)

	_, err := h.execute("create", "python:3.13")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `required flag(s) "name" not set`)
	assert.False(t, h.server.HasRequest("POST", "/builds"))
}

func TestSandboxBuildsCreate_RequiresSourceImage(t *testing.T) {
	h := newSandboxBuildsHarness(t)

	_, err := h.execute("create", "--name", "python")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestSandboxBuildsCreate_InvalidImageSurfacesAPIError(t *testing.T) {
	h := newSandboxBuildsHarness(t)

	_, err := h.execute("create", "Python:3.13", "--name", "python", "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "invalid sourceImage")
	assert.Empty(t, h.server.SandboxBuilds.Instances)
}

func TestSandboxBuildsCreate_ReturnsExistingBuildForSameNameAndImage(t *testing.T) {
	h := newSandboxBuildsHarness(t)
	existing := h.seedBuild(sandboxesclient.SandboxBuild{Name: "python"})

	result, err := h.execute("create", "python:3.13", "--name", "python", "--output", "json")
	require.NoError(t, err)

	body := testrequire.ParseJSONMap(t, result.Stdout)
	assert.Equal(t, existing.Id, body["id"])
	assert.Equal(t, "succeeded", body["status"])
	assert.Len(t, h.server.SandboxBuilds.Instances, 1, "no second build is started")
}

func TestSandboxBuildsCreate_LimitReached(t *testing.T) {
	h := newSandboxBuildsHarness(t)
	h.server.SandboxBuilds.RespondWith(http.StatusTooManyRequests)

	_, err := h.execute("create", "python:3.13", "--name", "python", "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many requests")
}

func TestSandboxBuildsCreate_UnknownGroup(t *testing.T) {
	h := newSandboxBuildsHarness(t)

	_, err := h.execute("create", "python:3.13", "--name", "python", "--group", testids.SandboxGroupID("missing"), "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "not found: sandbox group")
}

func TestSandboxBuildsCreate_GroupResolution(t *testing.T) {
	tests := []struct {
		name        string
		groups      []bool
		explicit    bool
		groupStatus int
		errContains string
	}{
		{name: "default group", groups: []bool{true}},
		{name: "default group after non-default group", groups: []bool{false, true}},
		{name: "explicit group skips lookup", groups: []bool{false}, explicit: true, groupStatus: http.StatusForbidden},
		{name: "no groups", errContains: "no default sandbox group"},
		{name: "only non-default groups", groups: []bool{false}, errContains: "no default sandbox group"},
		{name: "lookup error", groups: []bool{true}, groupStatus: http.StatusForbidden, errContains: "not allowed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxBuildsHarness(t)
			h.server.SandboxGroups.Instances = nil
			for _, isDefault := range tc.groups {
				group := h.server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{OwnerId: sandboxBuildsActiveWorkspaceID}))
				group.IsDefault = isDefault
				h.group = group
			}
			if tc.groupStatus != 0 {
				h.server.SandboxGroups.RespondWith(tc.groupStatus)
			}
			args := []string{"create", "python:3.13", "--name", "python", "--output", "json"}
			if tc.explicit {
				args = append(args, "--group", h.group.Id)
			}

			result, err := h.execute(args...)
			if tc.errContains != "" {
				require.ErrorContains(t, err, tc.errContains)
				assert.Empty(t, h.server.SandboxBuilds.Instances)
				return
			}
			require.NoError(t, err)
			body := testrequire.ParseJSONMap(t, result.Stdout)
			assert.Equal(t, h.group.Id, body["sandboxGroupId"])
		})
	}
}

func TestSandboxBuildsGet_TextOutput(t *testing.T) {
	h := newSandboxBuildsHarness(t)
	reason := "image is not linux/amd64"
	build := h.seedBuild(sandboxesclient.SandboxBuild{Name: "broken", Status: sandboxesclient.SandboxBuildStatusFailed, FailureReason: &reason})

	result, err := h.execute("get", build.Id, "--group", h.group.Id, "--output", "text")
	require.NoError(t, err)
	for _, want := range []string{build.Id, h.group.Id, "Name:           broken", "failed", "Failure reason: image is not linux/amd64", "Finished:       20"} {
		assert.Contains(t, result.Stdout, want)
	}
}

func TestSandboxBuildsGet_JSONOutput(t *testing.T) {
	h := newSandboxBuildsHarness(t)
	build := h.seedBuild(sandboxesclient.SandboxBuild{})

	result, err := h.execute("get", build.Id, "--group", h.group.Id, "--output", "json")
	require.NoError(t, err)
	body := testrequire.ParseJSONMap(t, result.Stdout)
	assert.Equal(t, build.Id, body["id"])
	assert.Equal(t, "succeeded", body["status"])
	assert.NotNil(t, body["finishedAt"])

	req, ok := h.server.LastRequest("GET", "/sandbox-groups/"+h.group.Id+"/builds/"+build.Id)
	require.True(t, ok, "expected a get request")
	assert.NotContains(t, req.URI, "ownerId")
}

func TestSandboxBuildsGet_NotFound(t *testing.T) {
	tests := []struct {
		name string
		seed func(h sandboxBuildsHarness) string
	}{
		{name: "unknown build", seed: func(sandboxBuildsHarness) string { return testids.SandboxBuildID("missing") }},
		{name: "build in another group", seed: func(h sandboxBuildsHarness) string {
			return h.seedBuild(sandboxesclient.SandboxBuild{SandboxGroupId: testids.SandboxGroupID("other")}).Id
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxBuildsHarness(t)
			buildID := tc.seed(h)

			_, err := h.execute("get", buildID, "--output", "json")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "404")
			assert.Contains(t, err.Error(), "not found: build")
		})
	}
}

func TestSandboxBuildsGet_GroupResolution(t *testing.T) {
	tests := []struct {
		name     string
		explicit bool
	}{
		{name: "default group"},
		{name: "explicit group", explicit: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxBuildsHarness(t)
			build := h.seedBuild(sandboxesclient.SandboxBuild{})
			args := []string{"get", build.Id, "--output", "json"}
			if tc.explicit {
				args = append(args, "--group", h.group.Id)
				h.server.SandboxGroups.RespondWith(http.StatusForbidden)
			}

			result, err := h.execute(args...)
			require.NoError(t, err)
			assert.Equal(t, build.Id, testrequire.ParseJSONMap(t, result.Stdout)["id"])
			assert.Equal(t, !tc.explicit, h.server.HasRequest("GET", "/sandbox-groups?"), "the group lookup runs only without --group")
		})
	}
}

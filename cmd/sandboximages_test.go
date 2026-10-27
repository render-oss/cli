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

var sandboxImagesActiveWorkspaceID = testids.WorkspaceID("images")

type sandboxImagesHarness struct {
	t      *testing.T
	server *renderapi.Server
	group  *sandboxesclient.SandboxGroup
}

func newSandboxImagesHarness(t *testing.T) sandboxImagesHarness {
	t.Helper()

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: sandboxImagesActiveWorkspaceID, Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", sandboxImagesActiveWorkspaceID)
	group := server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{OwnerId: sandboxImagesActiveWorkspaceID}))

	return sandboxImagesHarness{t: t, server: server, group: group}
}

func (h sandboxImagesHarness) execute(args ...string) (CommandResult, error) {
	h.t.Helper()
	return h.executeWithStdin("", args...)
}

func (h sandboxImagesHarness) executeWithStdin(stdin string, args ...string) (CommandResult, error) {
	h.t.Helper()
	return executeSandboxGroupsCommandWithStdin(h.t, h.server, stdin, append([]string{"sandbox-groups", "images"}, args...)...)
}

func (h sandboxImagesHarness) seedImage(b sandboxesclient.SandboxBuild) *sandboxesclient.SandboxBuild {
	if b.SandboxGroupId == "" {
		b.SandboxGroupId = h.group.Id
	}
	return h.server.SandboxBuilds.Add(renderapi.NewSandboxBuild(b))
}

func TestSandboxGroups_HelpListsImages(t *testing.T) {
	h := newSandboxImagesHarness(t)

	result, err := executeSandboxGroupsCommand(t, h.server, "sandbox-groups", "--help")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "images")
	assert.NotContains(t, result.Stdout, "builds")
}

func TestSandboxImages_HelpListsSubcommands(t *testing.T) {
	h := newSandboxImagesHarness(t)

	result, err := h.execute("--help")
	require.NoError(t, err)
	for _, sub := range []string{"import", "get"} {
		assert.Contains(t, result.Stdout, sub, "expected subcommand %q in help output", sub)
	}
}

func TestSandboxImagesImport_TextOutput(t *testing.T) {
	h := newSandboxImagesHarness(t)

	result, err := h.execute("import", "python:3.13", "--name", "python", "--output", "text")
	require.NoError(t, err)

	imported := h.server.SandboxBuilds.Only(t)
	for _, want := range []string{imported.Id, "Name:           python", "running", "docker.io/library/python:3.13", h.group.Id, "Finished:       -"} {
		assert.Contains(t, result.Stdout, want)
	}
	assert.NotContains(t, result.Stdout, "Failure reason")
	assert.False(t, h.server.HasRequest("GET", "/builds"), "import without --wait must return without polling")
}

func TestSandboxImagesImport_JSONOutput(t *testing.T) {
	h := newSandboxImagesHarness(t)

	result, err := h.execute("import", "python:3.13", "--name", "python", "--output", "json")
	require.NoError(t, err)

	imported := h.server.SandboxBuilds.Only(t)
	body := testrequire.ParseJSONMap(t, result.Stdout)
	assert.Equal(t, imported.Id, body["id"])
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

func TestSandboxImagesImport_RegistryCredentials(t *testing.T) {
	h := newSandboxImagesHarness(t)

	_, err := h.executeWithStdin("s3cret\n", "import", "ghcr.io/acme/agent:1.0", "--name", "agent",
		"--registry-username", "bot", "--registry-password-stdin", "--output", "json")
	require.NoError(t, err)

	req, ok := h.server.LastRequest("POST", "/sandbox-groups/"+h.group.Id+"/builds")
	require.True(t, ok, "expected a create request")
	body := testrequire.ParseJSONMap(t, string(req.Body))
	assert.Equal(t, map[string]any{"username": "bot", "password": "s3cret"}, body["registryCredential"])
}

func TestSandboxImagesImport_RegistryFlagsRequiredTogether(t *testing.T) {
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
			h := newSandboxImagesHarness(t)

			_, err := h.executeWithStdin(tc.stdin, append([]string{"import", "python:3.13", "--name", "python", "--output", "json"}, tc.args...)...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "registry-password-stdin")
			assert.False(t, h.server.HasRequest("POST", "/builds"))
		})
	}
}

func TestSandboxImagesImport_RequiresName(t *testing.T) {
	h := newSandboxImagesHarness(t)

	_, err := h.execute("import", "python:3.13")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `required flag(s) "name" not set`)
	assert.False(t, h.server.HasRequest("POST", "/builds"))
}

func TestSandboxImagesImport_RequiresSource(t *testing.T) {
	h := newSandboxImagesHarness(t)

	_, err := h.execute("import", "--name", "python")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestSandboxImagesImport_InvalidImageSurfacesAPIError(t *testing.T) {
	h := newSandboxImagesHarness(t)

	_, err := h.execute("import", "Python:3.13", "--name", "python", "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "invalid sourceImage")
	assert.Empty(t, h.server.SandboxBuilds.Instances)
}

func TestSandboxImagesImport_ReturnsExistingImageForSameNameAndDigest(t *testing.T) {
	h := newSandboxImagesHarness(t)
	existing := h.seedImage(sandboxesclient.SandboxBuild{Name: "python"})

	result, err := h.execute("import", "python:3.13", "--name", "python", "--output", "json")
	require.NoError(t, err)

	body := testrequire.ParseJSONMap(t, result.Stdout)
	assert.Equal(t, existing.Id, body["id"])
	assert.Equal(t, "succeeded", body["status"])
	assert.Len(t, h.server.SandboxBuilds.Instances, 1, "no second import is started")
}

func TestSandboxImagesImport_LimitReached(t *testing.T) {
	h := newSandboxImagesHarness(t)
	h.server.SandboxBuilds.RespondWith(http.StatusTooManyRequests)

	_, err := h.execute("import", "python:3.13", "--name", "python", "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many requests")
}

func TestSandboxImagesImport_UnknownGroup(t *testing.T) {
	h := newSandboxImagesHarness(t)

	_, err := h.execute("import", "python:3.13", "--name", "python", "--group", testids.SandboxGroupID("missing"), "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "not found: sandbox group")
}

func TestSandboxImagesImport_GroupResolution(t *testing.T) {
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
			h := newSandboxImagesHarness(t)
			h.server.SandboxGroups.Instances = nil
			for _, isDefault := range tc.groups {
				group := h.server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{OwnerId: sandboxImagesActiveWorkspaceID}))
				group.IsDefault = isDefault
				h.group = group
			}
			if tc.groupStatus != 0 {
				h.server.SandboxGroups.RespondWith(tc.groupStatus)
			}
			args := []string{"import", "python:3.13", "--name", "python", "--output", "json"}
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

func TestSandboxImagesGet_TextOutput(t *testing.T) {
	h := newSandboxImagesHarness(t)
	reason := "image is not linux/amd64"
	image := h.seedImage(sandboxesclient.SandboxBuild{Name: "broken", Status: sandboxesclient.SandboxBuildStatusFailed, FailureReason: &reason})

	result, err := h.execute("get", image.Id, "--group", h.group.Id, "--output", "text")
	require.NoError(t, err)
	for _, want := range []string{image.Id, h.group.Id, "Name:           broken", "failed", "Failure reason: image is not linux/amd64", "Finished:       20"} {
		assert.Contains(t, result.Stdout, want)
	}
}

func TestSandboxImagesGet_JSONOutput(t *testing.T) {
	h := newSandboxImagesHarness(t)
	image := h.seedImage(sandboxesclient.SandboxBuild{})

	result, err := h.execute("get", image.Id, "--group", h.group.Id, "--output", "json")
	require.NoError(t, err)
	body := testrequire.ParseJSONMap(t, result.Stdout)
	assert.Equal(t, image.Id, body["id"])
	assert.Equal(t, "succeeded", body["status"])
	assert.NotNil(t, body["finishedAt"])

	req, ok := h.server.LastRequest("GET", "/sandbox-groups/"+h.group.Id+"/builds/"+image.Id)
	require.True(t, ok, "expected a get request")
	assert.NotContains(t, req.URI, "ownerId")
}

func TestSandboxImagesGet_NotFound(t *testing.T) {
	tests := []struct {
		name string
		seed func(h sandboxImagesHarness) string
	}{
		{name: "unknown image", seed: func(sandboxImagesHarness) string { return testids.SandboxBuildID("missing") }},
		{name: "image in another group", seed: func(h sandboxImagesHarness) string {
			return h.seedImage(sandboxesclient.SandboxBuild{SandboxGroupId: testids.SandboxGroupID("other")}).Id
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxImagesHarness(t)
			imageID := tc.seed(h)

			_, err := h.execute("get", imageID, "--output", "json")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "404")
			assert.Contains(t, err.Error(), "not found: build")
		})
	}
}

func TestSandboxImagesGet_GroupResolution(t *testing.T) {
	tests := []struct {
		name     string
		explicit bool
	}{
		{name: "default group"},
		{name: "explicit group", explicit: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxImagesHarness(t)
			image := h.seedImage(sandboxesclient.SandboxBuild{})
			args := []string{"get", image.Id, "--output", "json"}
			if tc.explicit {
				args = append(args, "--group", h.group.Id)
				h.server.SandboxGroups.RespondWith(http.StatusForbidden)
			}

			result, err := h.execute(args...)
			require.NoError(t, err)
			assert.Equal(t, image.Id, testrequire.ParseJSONMap(t, result.Stdout)["id"])
			assert.Equal(t, !tc.explicit, h.server.HasRequest("GET", "/sandbox-groups?"), "the group lookup runs only without --group")
		})
	}
}

func TestSandboxImagesImport_WaitPollsUntilSucceeded(t *testing.T) {
	h := newSandboxImagesHarness(t)

	result, err := h.execute("import", "python:3.13", "--name", "python", "--wait", "--output", "json")
	require.NoError(t, err)

	imported := h.server.SandboxBuilds.Only(t)
	assert.Equal(t, "succeeded", testrequire.ParseJSONMap(t, result.Stdout)["status"])
	assert.True(t, h.server.HasRequest("GET", "/sandbox-groups/"+h.group.Id+"/builds/"+imported.Id), "--wait polls the new image")
}

func TestSandboxImagesImport_WaitFailurePrintsImage(t *testing.T) {
	for _, format := range []string{"json", "text"} {
		t.Run(format, func(t *testing.T) {
			h := newSandboxImagesHarness(t)
			h.server.SandboxBuilds.RespondToGetWith(http.StatusInternalServerError)

			result, err := h.execute("import", "python:3.13", "--name", "python", "--wait", "--output", format)
			require.Error(t, err)

			imported := h.server.SandboxBuilds.Only(t)
			assert.Contains(t, result.Stdout, imported.Id, "a failed wait still prints the image ID")
			assert.True(t, h.server.HasRequest("GET", "/builds/"+imported.Id), "--wait polls the new image")
		})
	}
}

// The deprecation notice goes to stderr in the CLI. The test harness sets the
// command's output, which cobra also uses for the notice, so it lands in
// Stdout here.
func TestSandboxGroupsBuilds_DeprecatedAliasesStillWork(t *testing.T) {
	h := newSandboxImagesHarness(t)

	result, err := executeSandboxGroupsCommand(t, h.server, "sandbox-groups", "builds", "create", "python:3.13", "--name", "python", "--output", "json")
	require.NoError(t, err)
	imported := h.server.SandboxBuilds.Only(t)
	assert.Contains(t, result.Stdout, imported.Id)
	assert.Contains(t, result.Stdout+result.Stderr, "use `render sandbox-groups images import` instead")

	result, err = executeSandboxGroupsCommand(t, h.server, "sandbox-groups", "builds", "get", imported.Id, "--output", "json")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, imported.Id)
	assert.Contains(t, result.Stdout+result.Stderr, "use `render sandbox-groups images get` instead")
	assert.True(t, h.server.HasRequest("GET", "/sandbox-groups/"+h.group.Id+"/builds/"+imported.Id))
}

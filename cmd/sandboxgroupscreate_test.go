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
	return executeSandboxGroupsCommand(h.t, h.server, append([]string{"sandbox-groups", "create"}, extraArgs...)...)
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

type sandboxGroupsCreateEnvOutcome struct {
	Err                 string
	Sent                bool
	SentEnvironmentID   string
	OutputEnvironmentID string
}

func sandboxGroupsCreateEnvOutcomeOf(
	t *testing.T,
	h sandboxGroupsCreateHarness,
	result CommandResult,
	err error,
) sandboxGroupsCreateEnvOutcome {
	t.Helper()

	got := sandboxGroupsCreateEnvOutcome{}
	if err != nil {
		got.Err = err.Error()
	}

	req, sent := h.server.LastRequest("POST", "/sandbox-groups")
	got.Sent = sent
	if sent {
		got.SentEnvironmentID = jsonStringField(testrequire.ParseJSONMap(t, string(req.Body)), "environmentId")
	}

	if err == nil {
		output := unmarshalSandboxGroupJSONOutput(t, result.Stdout)
		got.OutputEnvironmentID = jsonStringField(output, "environmentId")
	}

	return got
}

func TestSandboxGroupsCreate_Environment(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want sandboxGroupsCreateEnvOutcome
	}{
		{
			name: "omitted",
			args: []string{
				"sandbox-groups", "create",
				"--name", "agents",
				"--output", "json",
			},
			want: sandboxGroupsCreateEnvOutcome{
				Sent: true,
			},
		},
		{
			name: "by name",
			args: []string{
				"sandbox-groups", "create",
				"--name", "agents",
				"--environment", "staging",
				"--output", "json",
			},
			want: sandboxGroupsCreateEnvOutcome{
				Sent:                true,
				SentEnvironmentID:   sandboxGroupsStagingEnvID,
				OutputEnvironmentID: sandboxGroupsStagingEnvID,
			},
		},
		{
			name: "by ID",
			args: []string{
				"sandbox-groups", "create",
				"--name", "agents",
				"--environment", sandboxGroupsProductionEnvID,
				"--output", "json",
			},
			want: sandboxGroupsCreateEnvOutcome{
				Sent:                true,
				SentEnvironmentID:   sandboxGroupsProductionEnvID,
				OutputEnvironmentID: sandboxGroupsProductionEnvID,
			},
		},
		{
			name: "unknown name",
			args: []string{
				"sandbox-groups", "create",
				"--name", "agents",
				"--environment", "qa",
				"--output", "json",
			},
			want: sandboxGroupsCreateEnvOutcome{
				Err: `no environment found with name "qa"`,
			},
		},
		{
			name: "name in another workspace",
			args: []string{
				"sandbox-groups", "create",
				"--name", "agents",
				"--environment", "other",
				"--output", "json",
			},
			want: sandboxGroupsCreateEnvOutcome{
				Err: `no environment found with name "other"`,
			},
		},
		{
			name: "ID in another workspace",
			args: []string{
				"sandbox-groups", "create",
				"--name", "agents",
				"--environment", sandboxGroupsOtherEnvID,
				"--output", "json",
			},
			want: sandboxGroupsCreateEnvOutcome{
				Err: `environment "` + sandboxGroupsOtherEnvID + `"` +
					` belongs to workspace "` + sandboxGroupsOtherWorkspaceID + `",` +
					` not workspace "` + sandboxGroupsActiveWorkspaceID + `"`,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			h := newSandboxGroupsCreateHarness(t)
			seedSandboxGroupEnvironments(h.server)

			// act
			result, err := executeSandboxGroupsCommand(t, h.server, tc.args...)

			// assert
			assert.Equal(t, tc.want, sandboxGroupsCreateEnvOutcomeOf(t, h, result, err))
		})
	}
}

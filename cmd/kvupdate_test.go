package cmd

import (
	"encoding/json"
	"net/http"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeKVUpdate runs `render ea kv update <args>` against the fake server.
// Seeds and selects an active workspace before running the command.
func executeKVUpdate(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()
	t.Cleanup(resetKVUpdateFlags)
	resetKVUpdateFlags()

	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: ACTIVE_WORKSPACE_ID, Name: "Test Workspace"}))
	session := newCommandSession(t, server)
	if _, err := session.execute("workspace", "set", ACTIVE_WORKSPACE_ID, "--output", "text"); err != nil {
		return CommandResult{}, err
	}
	resetKVUpdateFlags()

	args := append([]string{"ea", "kv", "update"}, extraArgs...)
	return session.execute(args...)
}

func resetKVUpdateFlags() {
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "confirm" || f.Name == "output" {
			f.Changed = false
			f.Value.Set(f.DefValue) //nolint:errcheck
		}
	})
	kvUpdateCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		// pflag.SliceValue (stringArray/stringSlice) needs Replace, not Set —
		// Set would append "[]" as a literal element. Without Replace, values
		// from a previous test's --ip-allow-list flag persist into the next
		// test, which breaks any subsequent test that exercises the mutex
		// with --clear-ip-allow-list.
		if sliceVal, ok := f.Value.(pflag.SliceValue); ok {
			_ = sliceVal.Replace(nil)
			return
		}
		f.Value.Set(f.DefValue) //nolint:errcheck
	})
}

// TestKVUpdate_HappyPath_MultiField covers the non-interactive happy path end-to-end:
// targeting by ID, applying name + plan + memory-policy in one call, that the
// "queue" shortcut normalizes to noeviction at the cmd layer, that the server
// reflects every change, and that text output prints the new state.
func TestKVUpdate_HappyPath_MultiField(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "old-name")

	result, err := executeKVUpdate(t, server,
		kv.Id,
		"--name", "new-name",
		"--plan", "pro",
		"--memory-policy", "queue",
		"--output", "text",
	)
	require.NoError(t, err)

	// Server state reflects all three changes; queue shortcut resolved to noeviction.
	require.Len(t, server.KV.Instances, 1)
	updated := server.KV.Instances[0]
	assert.Equal(t, "new-name", updated.Name)
	assert.Equal(t, client.KeyValuePlanPro, updated.Plan)
	require.NotNil(t, updated.Options.MaxmemoryPolicy)
	assert.Equal(t, client.Noeviction, client.MaxmemoryPolicy(*updated.Options.MaxmemoryPolicy))

	// Text output anchors to the right KV and surfaces the fields text.KeyValueDetail renders.
	assert.Contains(t, result.Stdout, "Updated Key Value")
	assert.Contains(t, result.Stdout, kv.Id)
	assert.Contains(t, result.Stdout, "new-name")
	assert.Contains(t, result.Stdout, "pro")
}

func TestKVUpdate_NoMutatingFields_Errors(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "cache")
	_, err := executeKVUpdate(t, server, kv.Id, "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one field")
	assert.False(t, server.HasRequest("PATCH", "/key-value/"+kv.Id),
		"no PATCH should be issued when validation fails")
}

// --- Target resolution ---

// TestKVUpdate_ByName_UniqueMatch proves the positional argument can be a name
// (not just an ID) and that the lookup is scoped to the active workspace by default.
func TestKVUpdate_ByName_UniqueMatch(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "by-name-cache")

	result, err := executeKVUpdate(t, server, "by-name-cache", "--plan", "starter", "--output", "text")
	require.NoError(t, err)

	assert.Equal(t, kv.Id, server.KV.Instances[0].Id, "should have patched the by-name-cache instance")
	assert.Equal(t, client.KeyValuePlanStarter, server.KV.Instances[0].Plan)
	assert.Contains(t, result.Stdout, "Updated Key Value")
	assert.Contains(t, result.Stdout, kv.Id)
}

// TestKVUpdate_ResolveErrors_Propagate confirms that errors from the shared
// keyvalue.Resolve path flow through update without being swallowed. Resolve
// has its own deep coverage in pkg/keyvalue; one propagation check is enough.
func TestKVUpdate_ResolveErrors_Propagate(t *testing.T) {
	server := renderapi.NewServer(t)
	_, err := executeKVUpdate(t, server, "does-not-exist", "--plan", "free", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
	assert.Contains(t, err.Error(), "Test Workspace",
		"name errors should surface the active workspace from the resolve layer")
	assert.False(t, server.HasRequest("PATCH", "/key-value/"))
}

// TestKVUpdate_EnvironmentDisambiguatesName proves --environment actually scopes
// the name lookup when the same name exists in multiple environments. Without
// the flag, this would be ambiguous; with it, only the prod instance is patched.
func TestKVUpdate_EnvironmentDisambiguatesName(t *testing.T) {
	server := renderapi.NewServer(t)
	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: ACTIVE_WORKSPACE_ID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)
	prodKV := seedKVInEnv(server, "shared", project.Env("production").Id)
	stagingKV := seedKVInEnv(server, "shared", project.Env("staging").Id)

	_, err := executeKVUpdate(t, server,
		"shared",
		"--environment", "production",
		"--plan", "starter",
		"--output", "text",
	)
	require.NoError(t, err)

	for _, inst := range server.KV.Instances {
		switch inst.Id {
		case prodKV.Id:
			assert.Equal(t, client.KeyValuePlanStarter, inst.Plan, "prod KV should be patched")
		case stagingKV.Id:
			assert.NotEqual(t, client.KeyValuePlanStarter, inst.Plan, "staging KV should be untouched")
		}
	}
}

// --- IP allow-list ---

func TestKVUpdate_IPAllowList_Replace(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "cache")

	_, err := executeKVUpdate(t, server,
		kv.Id,
		"--ip-allow-list", "cidr=203.0.113.5/32,description=office",
		"--ip-allow-list", "cidr=10.0.0.0/8,description=internal",
		"--output", "text",
	)
	require.NoError(t, err)

	list := server.KV.Instances[0].IpAllowList
	require.Len(t, list, 2)
	assert.Equal(t, "203.0.113.5/32", list[0].CidrBlock)
	assert.Equal(t, "office", list[0].Description)
	assert.Equal(t, "10.0.0.0/8", list[1].CidrBlock)
	assert.Equal(t, "internal", list[1].Description)
}

// TestKVUpdate_ClearIPAllowList starts from a non-empty server-side list and
// proves --clear-ip-allow-list removes every entry (rather than leaving them
// in place, which would be the no-op result if the cmd sent nil instead of []).
func TestKVUpdate_ClearIPAllowList(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "cache")
	server.KV.Instances[0].IpAllowList = []client.CidrBlockAndDescription{
		{CidrBlock: "192.168.0.0/16", Description: "old"},
	}

	_, err := executeKVUpdate(t, server, kv.Id, "--clear-ip-allow-list", "--output", "text")
	require.NoError(t, err)

	assert.Empty(t, server.KV.Instances[0].IpAllowList)
}

func TestKVUpdate_IPAllowListFlags_MutuallyExclusive(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "cache")

	_, err := executeKVUpdate(t, server,
		kv.Id,
		"--ip-allow-list", "cidr=10.0.0.0/8",
		"--clear-ip-allow-list",
		"--output", "text",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--ip-allow-list")
	assert.Contains(t, err.Error(), "--clear-ip-allow-list")
	assert.False(t, server.HasRequest("PATCH", "/key-value/"+kv.Id))
}

// --- Output formats and API errors ---

// TestKVUpdate_OutputJSON proves --output json produces valid JSON of the
// post-update KV (matching the shape `kv create` returns), not the pre/post
// diff structure that text output renders.
func TestKVUpdate_OutputJSON(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "before-name")

	result, err := executeKVUpdate(t, server, kv.Id, "--name", "after-name", "--output", "json")
	require.NoError(t, err)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body),
		"expected valid JSON, got: %s", result.Stdout)
	assert.Equal(t, "after-name", body["name"])
}

// TestKVUpdate_APIError_Propagates proves a 5xx from the PATCH surfaces as
// a non-zero command exit.
func TestKVUpdate_APIError_Propagates(t *testing.T) {
	server := renderapi.NewServer(t)
	kv := seedKV(server, "cache")
	server.KV.RespondWith(http.StatusInternalServerError)

	_, err := executeKVUpdate(t, server, kv.Id, "--name", "renamed", "--output", "text")
	require.Error(t, err)
}

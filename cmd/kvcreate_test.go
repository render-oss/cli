package cmd

import (
	"encoding/json"
	"net/http"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeKVCreate runs `render ea kv create <extraArgs>` against the fake server.
// It seeds and selects a default workspace before running the create command.
// Pass --workspace explicitly in extraArgs for tests that exercise the workspace flag.
func executeKVCreate(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()
	t.Cleanup(resetKVCreateFlags)
	resetKVCreateFlags()

	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: ACTIVE_WORKSPACE_ID, Name: "Test Workspace"}))
	session := newCommandSession(t, server)
	if _, err := session.execute("workspace", "set", ACTIVE_WORKSPACE_ID, "--output", "text"); err != nil {
		return CommandResult{}, err
	}
	resetKVCreateFlags()

	args := append([]string{"ea", "kv", "create"}, extraArgs...)
	return session.execute(args...)
}

// resetKVCreateFlags resets all kvCreateCmd flags to their defaults.
// Cobra does not reset flag values between Execute() calls.
func resetKVCreateFlags() {
	kvCreateCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		// Skip array/slice flags: calling Set("[]") would append "[]" as an element.
		switch f.Value.Type() {
		case "stringArray", "stringSlice":
		default:
			f.Value.Set(f.DefValue) //nolint:errcheck
		}
	})
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "confirm" || f.Name == "output" {
			f.Changed = false
			f.Value.Set(f.DefValue) //nolint:errcheck
		}
	})
}

// --- Tests ---

func TestKVCreate_NonInteractive_AllFlags(t *testing.T) {
	server := renderapi.NewServer(t)
	result, err := executeKVCreate(t, server,
		"--name", "my-cache",
		"--plan", "starter",
		"--region", "virginia",
		"--memory-policy", "allkeys_lru",
		"--workspace", ACTIVE_WORKSPACE_ID,
		"--output", "text",
	)
	require.NoError(t, err)

	require.Len(t, server.KV.Instances, 1)
	kv := server.KV.Instances[0]
	assert.Equal(t, "my-cache", kv.Name)
	assert.Equal(t, client.KeyValuePlanStarter, kv.Plan)
	assert.Equal(t, ACTIVE_WORKSPACE_ID, kv.Owner.Id)
	assert.Equal(t, client.Virginia, kv.Region)
	require.NotNil(t, kv.Options.MaxmemoryPolicy)
	assert.Equal(t, client.AllkeysLru, client.MaxmemoryPolicy(*kv.Options.MaxmemoryPolicy))
	assert.Nil(t, kv.EnvironmentId)

	assert.Contains(t, result.Stdout, "my-cache")
	assert.Contains(t, result.Stdout, kv.Id)
}

func TestKVCreate_NonInteractive_DefaultsApplied(t *testing.T) {
	server := renderapi.NewServer(t)
	result, err := executeKVCreate(t, server,
		"--name", "my-kv",
		"--plan", "free",
		"--output", "text",
		// no --region, --memory-policy, --workspace flag, --environment
	)
	require.NoError(t, err)

	require.Len(t, server.KV.Instances, 1)
	kv := server.KV.Instances[0]
	assert.Equal(t, ACTIVE_WORKSPACE_ID, kv.Owner.Id)
	assert.Equal(t, client.Oregon, kv.Region)
	require.NotNil(t, kv.Options.MaxmemoryPolicy)
	assert.Equal(t, client.AllkeysLru, client.MaxmemoryPolicy(*kv.Options.MaxmemoryPolicy))
	assert.Nil(t, kv.EnvironmentId)
	assert.Contains(t, result.Stdout, "my-kv")
}

func TestKVCreate_IPAllowList(t *testing.T) {
	server := renderapi.NewServer(t)
	_, err := executeKVCreate(t, server,
		"--name", "my-kv",
		"--plan", "free",
		"--ip-allow-list", "cidr=203.0.113.5/32,description=office",
		"--ip-allow-list", "cidr=10.0.0.0/8,description=internal",
		"--output", "text",
	)
	require.NoError(t, err)

	require.Len(t, server.KV.Instances, 1)
	allowList := server.KV.Instances[0].IpAllowList
	require.Len(t, allowList, 2)
	assert.Equal(t, "203.0.113.5/32", allowList[0].CidrBlock)
	assert.Equal(t, "office", allowList[0].Description)
	assert.Equal(t, "10.0.0.0/8", allowList[1].CidrBlock)
	assert.Equal(t, "internal", allowList[1].Description)
}

func TestKVCreate_WorkspaceFlagOverridesActiveWorkspace(t *testing.T) {
	server := renderapi.NewServer(t)
	targetWorkspaceID := testids.WorkspaceID("target")
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: targetWorkspaceID, Name: "Target Workspace"}))

	_, err := executeKVCreate(t, server,
		"--name", "my-kv",
		"--plan", "free",
		"--workspace", "Target Workspace",
		"--output", "text",
	)
	require.NoError(t, err)
	require.Len(t, server.KV.Instances, 1)
	assert.Equal(t, targetWorkspaceID, server.KV.Instances[0].Owner.Id)
}

func TestKVCreate_WorkspaceByName_NoMatch(t *testing.T) {
	server := renderapi.NewServer(t)
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--workspace", "nonexistent-workspace",
		"--output", "text",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-workspace")
	assert.Empty(t, server.KV.Instances)
}

func TestKVCreate_Interactive_InvalidWorkspaceFailsBeforePrompt(t *testing.T) {
	server := renderapi.NewServer(t)
	missingWorkspaceID := testids.WorkspaceID("missing")

	// This does not attempt to drive the huh confirmation prompt. The invalid workspace
	// should fail during pre-prompt resolution; if the command reaches huh, this test
	// fails with the test environment's lack of a TTY instead of the expected workspace error.
	_, err := executeKVCreate(t, server,
		"--name", "my-kv",
		"--plan", "free",
		"--region", "oregon",
		"--memory-policy", "allkeys_lru",
		"--workspace", missingWorkspaceID,
		"--output", "interactive",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `workspace "`+missingWorkspaceID+`" not found`)
	assert.Empty(t, server.KV.Instances)
}

func TestKVCreate_WorkspaceByName_MultipleMatches(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: "tea-workspace-aaa", Name: "Shared Name", Email: "a@example.com"}))
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: "tea-workspace-bbb", Name: "Shared Name", Email: "b@example.com"}))
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--workspace", "Shared Name",
		"--output", "text",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace ID")
	assert.Empty(t, server.KV.Instances)
}

func TestKVCreate_WorkspaceByID(t *testing.T) {
	server := renderapi.NewServer(t)
	targetWorkspaceID := testids.WorkspaceID("target")
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: targetWorkspaceID, Name: "Target Workspace"}))

	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--workspace", targetWorkspaceID,
		"--output", "text",
	)
	require.NoError(t, err)
	assert.Equal(t, targetWorkspaceID, server.KV.Instances[0].Owner.Id)
	assert.True(t,
		server.HasRequest(http.MethodGet, "/owners/"+targetWorkspaceID),
		"expected direct owner retrieval by ID")
}

// These scope-resolution-heavy tests intentionally remain here as refactor
// safety for the KV create command. New shared scope behavior should be tested
// in pkg/resolve; command-level tests should focus on KV-specific policy.
func TestKVCreate_EnvironmentByID_DerivesWorkspaceFromEnvironmentProject(t *testing.T) {
	t.Cleanup(resetKVCreateFlags)
	server := renderapi.NewServer(t)
	inactiveWorkspace := renderapi.NewOwner(client.Owner{Id: testids.WorkspaceID("inactive"), Name: "Not the Active Workspace"})
	server.Owners.Add(inactiveWorkspace)
	projectID := testids.ProjectID("project")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "My Project",
		OwnerId: inactiveWorkspace.Id,
	}))
	environmentID := testids.EnvironmentID("production")
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        environmentID,
		Name:      "production",
		ProjectId: projectID,
	}))

	_, err := executeCommand(t, server,
		"ea", "kv", "create",
		"--name", "my-kv",
		"--plan", "free",
		"--environment", environmentID,
		"--output", "text",
	)
	require.NoError(t, err)
	require.Len(t, server.KV.Instances, 1)
	kv := server.KV.Instances[0]
	assert.Equal(t, inactiveWorkspace.Id, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, environmentID, *kv.EnvironmentId)
}

func TestKVCreate_EnvironmentByID_DerivesWorkspaceIgnoringActiveWorkspace(t *testing.T) {
	server := renderapi.NewServer(t)
	inactiveWorkspace := renderapi.NewOwner(client.Owner{Id: testids.WorkspaceID("inactive"), Name: "Not the Active Workspace"})
	server.Owners.Add(inactiveWorkspace)

	projectID := testids.ProjectID("project")

	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "Project with 1 Environment",
		OwnerId: inactiveWorkspace.Id,
	}))

	prodEnvID := testids.EnvironmentID("production")
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        prodEnvID,
		Name:      "production",
		ProjectId: projectID,
	}))

	_, err := executeKVCreate(t, server,
		"--name", "my-kv",
		"--plan", "free",
		"--environment", prodEnvID,
		"--output", "text",
	)
	require.NoError(t, err)
	require.Len(t, server.KV.Instances, 1)
	kv := server.KV.Instances[0]
	assert.Equal(t, inactiveWorkspace.Id, kv.Owner.Id)
	assert.NotEqual(t, ACTIVE_WORKSPACE_ID, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, prodEnvID, *kv.EnvironmentId)
}

func TestKVCreate_ProjectFlagDerivesWorkspaceIgnoringActiveWorkspace(t *testing.T) {
	server := renderapi.NewServer(t)
	projectWorkspace := renderapi.NewOwner(client.Owner{Id: testids.WorkspaceID("project workspace"), Name: "Project Workspace"})
	server.Owners.Add(projectWorkspace)

	projectID := testids.ProjectID("project workspace")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "Project Workspace App",
		OwnerId: projectWorkspace.Id,
	}))

	environmentID := testids.EnvironmentID("project workspace")
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        environmentID,
		Name:      "production",
		ProjectId: projectID,
	}))

	_, err := executeKVCreate(t, server,
		"--name", "my-kv",
		"--plan", "free",
		"--project", projectID,
		"--output", "text",
	)
	require.NoError(t, err)
	require.Len(t, server.KV.Instances, 1)
	kv := server.KV.Instances[0]
	assert.Equal(t, projectWorkspace.Id, kv.Owner.Id)
	assert.NotEqual(t, ACTIVE_WORKSPACE_ID, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, environmentID, *kv.EnvironmentId)
}

func TestKVCreate_EnvironmentByID_WorkspaceFlagMismatchErrors(t *testing.T) {
	t.Cleanup(resetKVCreateFlags)
	server := renderapi.NewServer(t)
	workspace1 := renderapi.NewOwner(client.Owner{Id: testids.WorkspaceID("from flag"), Name: "Workspace 1"})
	workspace2 := renderapi.NewOwner(client.Owner{Id: testids.WorkspaceID("other workspace"), Name: "Workspace 2"})
	server.Owners.Add(workspace1)
	server.Owners.Add(workspace2)

	projectID := testids.ProjectID("workspace two")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "Project Belonging to Workspace 2",
		OwnerId: workspace2.Id,
	}))
	environmentID := testids.EnvironmentID("workspace two")
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        environmentID,
		Name:      "Environment Inside Workspace 2",
		ProjectId: projectID,
	}))

	_, err := executeCommand(t, server,
		"ea", "kv", "create",
		"--name", "my-kv",
		"--plan", "free",
		"--workspace", workspace1.Id,
		"--environment", environmentID,
		"--output", "text",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment")
	assert.Contains(t, err.Error(), "workspace")
	assert.Empty(t, server.KV.Instances)
}

func TestKVCreate_EnvironmentByID_ProjectFlagMismatchErrors(t *testing.T) {
	server := renderapi.NewServer(t)

	project1ID := testids.ProjectID("one")
	project2ID := testids.ProjectID("two")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      project1ID,
		Name:    "Project 1",
		OwnerId: ACTIVE_WORKSPACE_ID,
	}))
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      project2ID,
		Name:    "Project 2",
		OwnerId: ACTIVE_WORKSPACE_ID,
	}))
	environmentID := testids.EnvironmentID("project two")
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        environmentID,
		Name:      "Environment in Project 2",
		ProjectId: project2ID,
	}))

	_, err := executeKVCreate(t, server,
		"--name", "my-kv",
		"--plan", "free",
		"--project", project1ID,
		"--environment", environmentID,
		"--output", "text",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment")
	assert.Contains(t, err.Error(), "project")
	assert.Empty(t, server.KV.Instances)
}

func TestKVCreate_EnvironmentByName_UniqueMatch(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("project")
	environmentID := testids.EnvironmentID("production")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: projectID, Name: "My Project", OwnerId: ACTIVE_WORKSPACE_ID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: environmentID, Name: "production", ProjectId: projectID}))
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--environment", "production",
		"--output", "text",
	)
	require.NoError(t, err)
	require.Len(t, server.KV.Instances, 1)
	kv := server.KV.Instances[0]
	assert.Equal(t, ACTIVE_WORKSPACE_ID, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, environmentID, *kv.EnvironmentId)
}

func TestKVCreate_EnvironmentByName_NoMatch(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: testids.ProjectID("project"), Name: "My Project", OwnerId: ACTIVE_WORKSPACE_ID}))
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--environment", "nonexistent",
		"--output", "text",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Empty(t, server.KV.Instances)
}

func TestKVCreate_EnvironmentByName_AmbiguousMatch(t *testing.T) {
	server := renderapi.NewServer(t)
	projectAID := testids.ProjectID("project a")
	projectBID := testids.ProjectID("project b")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: projectAID, Name: "Project A", OwnerId: ACTIVE_WORKSPACE_ID}))
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: projectBID, Name: "Project B", OwnerId: ACTIVE_WORKSPACE_ID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: testids.EnvironmentID("a"), Name: "production", ProjectId: projectAID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: testids.EnvironmentID("b"), Name: "production", ProjectId: projectBID}))
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--environment", "production",
		"--output", "text",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment ID")
	assert.Empty(t, server.KV.Instances)
}

func TestKVCreate_ConfirmMode_GeneratesName(t *testing.T) {
	server := renderapi.NewServer(t)
	_, err := executeKVCreate(t, server,
		"--plan", "free",
		"--confirm",
	)
	require.NoError(t, err)
	require.Len(t, server.KV.Instances, 1)
	kv := server.KV.Instances[0]
	// Generated name is a petname like "happy-lion" (no kv- prefix)
	assert.NotEmpty(t, kv.Name)
	assert.Contains(t, kv.Name, "-",
		"expected generated petname with hyphen separator, got %q", kv.Name)
}

func TestKVCreate_OutputJSON(t *testing.T) {
	server := renderapi.NewServer(t)
	cmdResult, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--output", "json",
	)
	require.NoError(t, err)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(cmdResult.Stdout), &result), "expected valid JSON, got: %s", cmdResult.Stdout)
	assert.Equal(t, "my-kv", result["name"])
}

func TestKVCreate_OutputYAML(t *testing.T) {
	server := renderapi.NewServer(t)
	result, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--output", "yaml",
	)
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "name:")
	assert.Contains(t, result.Stdout, "my-kv")
}

func TestKVCreate_InvalidRegion(t *testing.T) {
	server := renderapi.NewServer(t)
	result, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free", "--region", "mars",
		"--output", "text",
	)
	require.Error(t, err)
	assert.Contains(t, result.Stderr, `invalid argument "mars" for "--region" flag`)
	assert.Contains(t, result.Stderr, `"oregon"`)
	assert.Contains(t, result.Stderr, `"virginia"`)
	assert.Contains(t, result.Stdout, "Usage:")
	assert.Contains(t, result.Stdout, "Region: frankfurt | ohio | oregon | singapore | virginia")
	assert.Empty(t, server.KV.Instances)
}

func TestKVCreate_InvalidMemoryPolicy(t *testing.T) {
	server := renderapi.NewServer(t)
	result, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free", "--memory-policy", "delete-everything",
		"--output", "text",
	)
	require.Error(t, err)
	assert.Contains(t, result.Stderr, `invalid argument "delete-everything" for "--memory-policy" flag`)
	assert.Contains(t, result.Stderr, `"cache"`)
	assert.Contains(t, result.Stderr, `"queue"`)
	assert.Contains(t, result.Stderr, `"noeviction"`)
	assert.Contains(t, result.Stdout, "Usage:")
	assert.Contains(t, result.Stdout, "Shortcuts: cache")
	assert.Contains(t, result.Stdout, "Technical values: noeviction")
	assert.Empty(t, server.KV.Instances)
}

func TestKVCreate_MemoryPolicyCache_Shortcut(t *testing.T) {
	server := renderapi.NewServer(t)
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--memory-policy", "cache",
		"--output", "text",
	)
	require.NoError(t, err)
	kv := server.KV.Instances[0]
	require.NotNil(t, kv.Options.MaxmemoryPolicy)
	assert.Equal(t, client.AllkeysLru, client.MaxmemoryPolicy(*kv.Options.MaxmemoryPolicy),
		"cache shortcut should normalize to allkeys_lru")
}

func TestKVCreate_MemoryPolicyQueue_Shortcut(t *testing.T) {
	server := renderapi.NewServer(t)
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--memory-policy", "queue",
		"--output", "text",
	)
	require.NoError(t, err)
	kv := server.KV.Instances[0]
	require.NotNil(t, kv.Options.MaxmemoryPolicy)
	assert.Equal(t, client.Noeviction, client.MaxmemoryPolicy(*kv.Options.MaxmemoryPolicy),
		"queue shortcut should normalize to noeviction")
}

func TestKVCreate_NoWorkspaceConfigured(t *testing.T) {
	t.Cleanup(resetKVCreateFlags)
	server := renderapi.NewServer(t)
	result, err := executeCommand(t, server,
		"ea", "kv", "create", "--name", "my-kv", "--plan", "free", "--output", "text",
	)
	require.Error(t, err)
	assert.Contains(t, result.Stderr, "no workspace")
	assert.Empty(t, server.KV.Instances)
}

func TestKVCreate_APIError(t *testing.T) {
	server := renderapi.NewServer(t)
	server.KV.RespondWith(http.StatusInternalServerError)
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free", "--output", "text",
	)
	require.Error(t, err)
	assert.Empty(t, server.KV.Instances)
}

func TestKVCreate_ProjectByID_SingleEnv(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("project")
	environmentID := testids.EnvironmentID("production")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "My Project",
		OwnerId: ACTIVE_WORKSPACE_ID,
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: environmentID, Name: "production", ProjectId: projectID}))
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--project", projectID,
		"--output", "text",
	)
	require.NoError(t, err)
	kv := server.KV.Instances[0]
	assert.Equal(t, ACTIVE_WORKSPACE_ID, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, environmentID, *kv.EnvironmentId)
}

func TestKVCreate_ProjectByName_SingleEnv(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("project")
	environmentID := testids.EnvironmentID("production")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "My Project",
		OwnerId: ACTIVE_WORKSPACE_ID,
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: environmentID, Name: "production", ProjectId: projectID}))
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--project", "My Project",
		"--output", "text",
	)
	require.NoError(t, err)
	kv := server.KV.Instances[0]
	assert.Equal(t, ACTIVE_WORKSPACE_ID, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, environmentID, *kv.EnvironmentId)
}

func TestKVCreate_ProjectFlag_MultipleEnvs_Error(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("project")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "My Project",
		OwnerId: ACTIVE_WORKSPACE_ID,
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: testids.EnvironmentID("staging"), Name: "staging", ProjectId: projectID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: testids.EnvironmentID("production"), Name: "production", ProjectId: projectID}))
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--project", projectID,
		"--output", "text",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "staging")
	assert.Contains(t, err.Error(), "production")
	assert.Contains(t, err.Error(), "--environment")
	assert.Empty(t, server.KV.Instances)
}

// TestKVCreate_ProjectByID_DisambiguatesAmbiguousEnvironmentName verifies that
// --project scopes the environment search when an environment name appears in
// multiple projects. Without --project, "production" would be ambiguous across
// Project A and Project B; with --project set to Project A's ID it resolves
// unambiguously.
func TestKVCreate_ProjectByID_DisambiguatesAmbiguousEnvironmentName(t *testing.T) {
	server := renderapi.NewServer(t)
	projectAID := testids.ProjectID("project a")
	projectBID := testids.ProjectID("project b")
	environmentAID := testids.EnvironmentID("project a")
	environmentBID := testids.EnvironmentID("project b")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: projectAID, Name: "Project A", OwnerId: ACTIVE_WORKSPACE_ID}))
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: projectBID, Name: "Project B", OwnerId: ACTIVE_WORKSPACE_ID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: environmentAID, Name: "production", ProjectId: projectAID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: environmentBID, Name: "production", ProjectId: projectBID}))
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--project", projectAID,
		"--environment", "production",
		"--output", "text",
	)
	require.NoError(t, err)
	kv := server.KV.Instances[0]
	assert.Equal(t, ACTIVE_WORKSPACE_ID, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, environmentAID, *kv.EnvironmentId)
}

// TestKVCreate_ProjectByName_DisambiguatesAmbiguousEnvironmentName is the same
// scenario as TestKVCreate_ProjectByID_DisambiguatesAmbiguousEnvironmentName but
// passes --project as a human-readable name rather than a prj- ID, exercising
// the name-resolution path in resolveProjectID.
func TestKVCreate_ProjectByName_DisambiguatesAmbiguousEnvironmentName(t *testing.T) {
	server := renderapi.NewServer(t)
	projectAID := testids.ProjectID("project a")
	projectBID := testids.ProjectID("project b")
	environmentAID := testids.EnvironmentID("project a")
	environmentBID := testids.EnvironmentID("project b")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: projectAID, Name: "Project A", OwnerId: ACTIVE_WORKSPACE_ID}))
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{Id: projectBID, Name: "Project B", OwnerId: ACTIVE_WORKSPACE_ID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: environmentAID, Name: "production", ProjectId: projectAID}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{Id: environmentBID, Name: "production", ProjectId: projectBID}))
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--project", "Project A",
		"--environment", "production",
		"--output", "text",
	)
	require.NoError(t, err)
	kv := server.KV.Instances[0]
	assert.Equal(t, ACTIVE_WORKSPACE_ID, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, environmentAID, *kv.EnvironmentId)
}

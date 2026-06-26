package cmd

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeKVCreate runs `render ea kv create <extraArgs>` against the fake server.
// It seeds and selects a default workspace before running the create command.
// Pass --workspace explicitly in extraArgs for tests that exercise the workspace flag.
func executeKVCreate(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()

	args := append([]string{"create"}, extraArgs...)
	return executeKVCommand(t, server, args...)
}

// --- Tests ---

func TestKVCreate_NonInteractive_AllFlags(t *testing.T) {
	server := renderapi.NewServer(t)
	result, err := executeKVCreate(t, server,
		"--name", "my-cache",
		"--plan", "starter",
		"--region", "virginia",
		"--memory-policy", "allkeys_lru",
		"--workspace", kvTestWorkspaceID,
		"--output", "text",
	)
	require.NoError(t, err)

	require.Len(t, server.KV.Instances, 1)
	kv := server.KV.Instances[0]
	assert.Equal(t, "my-cache", kv.Name)
	assert.Equal(t, client.KeyValuePlanStarter, kv.Plan)
	assert.Equal(t, kvTestWorkspaceID, kv.Owner.Id)
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
	assert.Equal(t, kvTestWorkspaceID, kv.Owner.Id)
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
	server := renderapi.NewServer(t)
	inactiveWorkspace := renderapi.NewOwner(client.Owner{Id: testids.WorkspaceID("inactive"), Name: "Not the Active Workspace"})
	server.Owners.Add(inactiveWorkspace)
	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: inactiveWorkspace.Id},
		renderapi.EnvAttrs{Name: "production"},
	)

	_, err := executeKVCommandWithoutActiveWorkspace(t, server,
		"create",
		"--name", "my-kv",
		"--plan", "free",
		"--environment", project.Env("production").Id,
		"--output", "text",
	)
	require.NoError(t, err)
	require.Len(t, server.KV.Instances, 1)
	kv := server.KV.Instances[0]
	assert.Equal(t, inactiveWorkspace.Id, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, project.Env("production").Id, *kv.EnvironmentId)
}

func TestKVCreate_EnvironmentByID_DerivesWorkspaceIgnoringActiveWorkspace(t *testing.T) {
	server := renderapi.NewServer(t)
	inactiveWorkspace := renderapi.NewOwner(client.Owner{Id: testids.WorkspaceID("inactive"), Name: "Not the Active Workspace"})
	server.Owners.Add(inactiveWorkspace)

	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project with 1 Environment", OwnerId: inactiveWorkspace.Id},
		renderapi.EnvAttrs{Name: "production"},
	)

	_, err := executeKVCreate(t, server,
		"--name", "my-kv",
		"--plan", "free",
		"--environment", project.Env("production").Id,
		"--output", "text",
	)
	require.NoError(t, err)
	require.Len(t, server.KV.Instances, 1)
	kv := server.KV.Instances[0]
	assert.Equal(t, inactiveWorkspace.Id, kv.Owner.Id)
	assert.NotEqual(t, kvTestWorkspaceID, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, project.Env("production").Id, *kv.EnvironmentId)
}

func TestKVCreate_ProjectFlagDerivesWorkspaceIgnoringActiveWorkspace(t *testing.T) {
	server := renderapi.NewServer(t)
	projectWorkspace := renderapi.NewOwner(client.Owner{Id: testids.WorkspaceID("project workspace"), Name: "Project Workspace"})
	server.Owners.Add(projectWorkspace)

	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project Workspace App", OwnerId: projectWorkspace.Id},
		renderapi.EnvAttrs{Name: "production"},
	)

	_, err := executeKVCreate(t, server,
		"--name", "my-kv",
		"--plan", "free",
		"--project", project.Project.Id,
		"--output", "text",
	)
	require.NoError(t, err)
	require.Len(t, server.KV.Instances, 1)
	kv := server.KV.Instances[0]
	assert.Equal(t, projectWorkspace.Id, kv.Owner.Id)
	assert.NotEqual(t, kvTestWorkspaceID, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, project.Env("production").Id, *kv.EnvironmentId)
}

func TestKVCreate_EnvironmentByID_WorkspaceFlagMismatchErrors(t *testing.T) {
	server := renderapi.NewServer(t)
	workspace1 := renderapi.NewOwner(client.Owner{Id: testids.WorkspaceID("from flag"), Name: "Workspace 1"})
	workspace2 := renderapi.NewOwner(client.Owner{Id: testids.WorkspaceID("other workspace"), Name: "Workspace 2"})
	server.Owners.Add(workspace1)
	server.Owners.Add(workspace2)

	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project Belonging to Workspace 2", OwnerId: workspace2.Id},
		renderapi.EnvAttrs{Name: "Environment Inside Workspace 2"},
	)

	_, err := executeKVCommandWithoutActiveWorkspace(t, server,
		"create",
		"--name", "my-kv",
		"--plan", "free",
		"--workspace", workspace1.Id,
		"--environment", project.Env("Environment Inside Workspace 2").Id,
		"--output", "text",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment")
	assert.Contains(t, err.Error(), "workspace")
	assert.Empty(t, server.KV.Instances)
}

func TestKVCreate_EnvironmentByID_ProjectFlagMismatchErrors(t *testing.T) {
	server := renderapi.NewServer(t)

	project1 := server.CreateProject(renderapi.ProjectAttrs{Name: "Project 1", OwnerId: kvTestWorkspaceID})
	project2 := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project 2", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "Environment in Project 2"},
	)

	_, err := executeKVCreate(t, server,
		"--name", "my-kv",
		"--plan", "free",
		"--project", project1.Project.Id,
		"--environment", project2.Env("Environment in Project 2").Id,
		"--output", "text",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment")
	assert.Contains(t, err.Error(), "project")
	assert.Empty(t, server.KV.Instances)
}

func TestKVCreate_EnvironmentByName_UniqueMatch(t *testing.T) {
	server := renderapi.NewServer(t)
	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--environment", "production",
		"--output", "text",
	)
	require.NoError(t, err)
	require.Len(t, server.KV.Instances, 1)
	kv := server.KV.Instances[0]
	assert.Equal(t, kvTestWorkspaceID, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, project.Env("production").Id, *kv.EnvironmentId)
}

func TestKVCreate_EnvironmentByName_NoMatch(t *testing.T) {
	server := renderapi.NewServer(t)
	server.CreateProject(renderapi.ProjectAttrs{Name: "My Project", OwnerId: kvTestWorkspaceID})
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
	server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project A", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project B", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
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
	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	env := project.Env("production")

	cmdResult, err := executeKVCreate(t, server,
		"--name", "my-kv",
		"--plan", "free",
		"--project", "My Project",
		"--environment", "production",
		"--memory-policy", "queue",
		"--ip-allow-list", "cidr=203.0.113.5/32,description=office",
		"--output", "json",
	)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(cmdResult.Stdout), &body), "expected valid JSON, got: %s", cmdResult.Stdout)
	data := requireSubMap(t, body, "data")
	require.Len(t, server.KV.Instances, 1)
	kv := server.KV.Instances[0]
	assert.Equal(t, map[string]any{
		"id":            kv.Id,
		"name":          "my-kv",
		"plan":          "free",
		"region":        "oregon",
		"status":        "available",
		"createdAt":     kv.CreatedAt.Format(time.RFC3339Nano),
		"updatedAt":     kv.UpdatedAt.Format(time.RFC3339Nano),
		"ownerId":       kvTestWorkspaceID,
		"projectId":     project.Project.Id,
		"environmentId": env.Id,
		"ipAllowList": []any{
			map[string]any{
				"cidrBlock":   "203.0.113.5/32",
				"description": "office",
			},
		},
		"maxmemoryPolicy": "noeviction",
	}, data)
}

func TestKVCreate_OutputYAML(t *testing.T) {
	server := renderapi.NewServer(t)
	result, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--output", "yaml",
	)
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "data:")
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
	assert.Contains(t, result.Stdout, "Set the region: frankfurt | ohio | oregon | singapore | virginia")
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
	assert.Contains(t, result.Stdout, "Accepts a friendly alias — cache")
	assert.Contains(t, result.Stdout, "or any raw policy: noeviction")
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
	server := renderapi.NewServer(t)
	result, err := executeKVCommandWithoutActiveWorkspace(t, server,
		"create", "--name", "my-kv", "--plan", "free", "--output", "text",
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
	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--project", project.Project.Id,
		"--output", "text",
	)
	require.NoError(t, err)
	kv := server.KV.Instances[0]
	assert.Equal(t, kvTestWorkspaceID, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, project.Env("production").Id, *kv.EnvironmentId)
}

func TestKVCreate_ProjectByName_SingleEnv(t *testing.T) {
	server := renderapi.NewServer(t)
	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--project", "My Project",
		"--output", "text",
	)
	require.NoError(t, err)
	kv := server.KV.Instances[0]
	assert.Equal(t, kvTestWorkspaceID, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, project.Env("production").Id, *kv.EnvironmentId)
}

func TestKVCreate_ProjectFlag_MultipleEnvs_Error(t *testing.T) {
	server := renderapi.NewServer(t)
	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "staging"},
		renderapi.EnvAttrs{Name: "production"},
	)
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--project", project.Project.Id,
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
	projectA := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project A", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project B", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--project", projectA.Project.Id,
		"--environment", "production",
		"--output", "text",
	)
	require.NoError(t, err)
	kv := server.KV.Instances[0]
	assert.Equal(t, kvTestWorkspaceID, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, projectA.Env("production").Id, *kv.EnvironmentId)
}

// TestKVCreate_ProjectByName_DisambiguatesAmbiguousEnvironmentName is the same
// scenario as TestKVCreate_ProjectByID_DisambiguatesAmbiguousEnvironmentName but
// passes --project as a human-readable name rather than a prj- ID, exercising
// the name-resolution path in resolveProjectID.
func TestKVCreate_ProjectByName_DisambiguatesAmbiguousEnvironmentName(t *testing.T) {
	server := renderapi.NewServer(t)
	projectA := server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project A", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	server.CreateProject(
		renderapi.ProjectAttrs{Name: "Project B", OwnerId: kvTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	_, err := executeKVCreate(t, server,
		"--name", "my-kv", "--plan", "free",
		"--project", "Project A",
		"--environment", "production",
		"--output", "text",
	)
	require.NoError(t, err)
	kv := server.KV.Instances[0]
	assert.Equal(t, kvTestWorkspaceID, kv.Owner.Id)
	require.NotNil(t, kv.EnvironmentId)
	assert.Equal(t, projectA.Env("production").Id, *kv.EnvironmentId)
}

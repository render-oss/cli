package resolve

import (
	"context"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stringPtr(s string) *string {
	return &s
}

func newTestResolver(t *testing.T, server *renderapi.Server) *Resolver {
	t.Helper()

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	return NewFromClient(c)
}

func seedOwner(server *renderapi.Server, id string) {
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: id, Name: id}))
}

func TestResolverResolveWorkspaceID_ByName(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: "tea-target", Name: "Target Workspace"}))
	resolver := newTestResolver(t, server)

	workspaceID, err := resolver.ResolveWorkspaceID(context.Background(), "Target Workspace")

	require.NoError(t, err)
	assert.Equal(t, "tea-target", workspaceID)
}

func TestResolverResolveWorkspaceID_PrefixLookingNameNotTreatedAsID(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: "tea-target", Name: "tea-target"}))
	resolver := newTestResolver(t, server)

	workspaceID, err := resolver.ResolveWorkspaceID(context.Background(), "tea-target")

	require.NoError(t, err)
	assert.Equal(t, "tea-target", workspaceID)
}

func TestResolverResolveProject_FiltersNameMatchesByWorkspace(t *testing.T) {
	server := renderapi.NewServer(t)
	seedOwner(server, "tea-first")
	seedOwner(server, "tea-second")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      "prj-first",
		Name:    "Shared Project",
		OwnerId: "tea-first",
	}))
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      "prj-second",
		Name:    "Shared Project",
		OwnerId: "tea-second",
	}))
	resolver := newTestResolver(t, server)

	project, err := resolver.ResolveProject(context.Background(), "Shared Project", "tea-second")

	require.NoError(t, err)
	assert.Equal(t, "prj-second", project.Id)
	assert.Equal(t, "tea-second", project.Owner.Id)
	assert.True(t, server.HasRequest("GET", "ownerId=tea-second"))
}

func TestResolverResolveProject_PrefixLookingNameNotTreatedAsID(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("prefix name project")
	seedOwner(server, "tea-owner")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "prj-short-name",
		OwnerId: "tea-owner",
	}))
	resolver := newTestResolver(t, server)

	project, err := resolver.ResolveProject(context.Background(), "prj-short-name", "")

	require.NoError(t, err)
	assert.Equal(t, projectID, project.Id)
}

func TestResolverResolveEnvironmentID_ScopesEnvironmentNameToProject(t *testing.T) {
	server := renderapi.NewServer(t)
	firstProjectID := testids.ProjectID("first")
	secondProjectID := testids.ProjectID("second")
	firstEnvironmentID := testids.EnvironmentID("first")
	secondEnvironmentID := testids.EnvironmentID("second")
	seedOwner(server, "tea-owner")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      firstProjectID,
		Name:    "First Project",
		OwnerId: "tea-owner",
	}))
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      secondProjectID,
		Name:    "Second Project",
		OwnerId: "tea-owner",
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        firstEnvironmentID,
		Name:      "production",
		ProjectId: firstProjectID,
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        secondEnvironmentID,
		Name:      "production",
		ProjectId: secondProjectID,
	}))
	resolver := newTestResolver(t, server)
	envName := "production"
	project := &client.Project{Id: secondProjectID}

	environmentID, err := resolver.ResolveEnvironmentID(context.Background(), project, &envName, "")

	require.NoError(t, err)
	require.NotNil(t, environmentID)
	assert.Equal(t, secondEnvironmentID, *environmentID)
}

func TestResolverResolveScope_PrefixLookingEnvironmentNameNotTreatedAsID(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("prefix env project")
	environmentID := testids.EnvironmentID("prefix env")
	seedOwner(server, "tea-owner")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "My Project",
		OwnerId: "tea-owner",
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        environmentID,
		Name:      "evm-short-name",
		ProjectId: projectID,
	}))
	resolver := newTestResolver(t, server)

	scope, err := resolver.ResolveScope(context.Background(), ScopeInput{
		EnvironmentIDOrName: stringPtr("evm-short-name"),
	})

	require.NoError(t, err)
	require.NotNil(t, scope.Environment)
	assert.Equal(t, environmentID, scope.Environment.Id)
}

func TestResolverResolveScope_ProjectImpliesWorkspace(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("project")
	environmentID := testids.EnvironmentID("production")
	seedOwner(server, "tea-project-owner")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "My Project",
		OwnerId: "tea-project-owner",
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        environmentID,
		Name:      "production",
		ProjectId: projectID,
	}))
	resolver := newTestResolver(t, server)

	scope, err := resolver.ResolveScope(context.Background(), ScopeInput{
		ProjectIDOrName: stringPtr(projectID),
	})

	require.NoError(t, err)
	assert.Equal(t, "tea-project-owner", scope.WorkspaceID)
	require.NotNil(t, scope.Project)
	assert.Equal(t, projectID, scope.Project.Id)
	assert.Nil(t, scope.Environment)
}

func TestResolverResolveScope_EnvironmentIDImpliesProjectAndWorkspace(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("project")
	environmentID := testids.EnvironmentID("production")
	seedOwner(server, "tea-project-owner")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "My Project",
		OwnerId: "tea-project-owner",
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        environmentID,
		Name:      "production",
		ProjectId: projectID,
	}))
	resolver := newTestResolver(t, server)

	scope, err := resolver.ResolveScope(context.Background(), ScopeInput{
		EnvironmentIDOrName: stringPtr(environmentID),
	})

	require.NoError(t, err)
	assert.Equal(t, "tea-project-owner", scope.WorkspaceID)
	require.NotNil(t, scope.Project)
	assert.Equal(t, projectID, scope.Project.Id)
	require.NotNil(t, scope.Environment)
	assert.Equal(t, environmentID, scope.Environment.Id)
}

func TestResolverResolveScope_BroadEnvironmentNameSearch(t *testing.T) {
	server := renderapi.NewServer(t)
	firstProjectID := testids.ProjectID("first")
	secondProjectID := testids.ProjectID("second")
	stagingEnvironmentID := testids.EnvironmentID("staging")
	productionEnvironmentID := testids.EnvironmentID("production")
	seedOwner(server, "tea-owner")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      firstProjectID,
		Name:    "First Project",
		OwnerId: "tea-owner",
	}))
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      secondProjectID,
		Name:    "Second Project",
		OwnerId: "tea-owner",
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        stagingEnvironmentID,
		Name:      "staging",
		ProjectId: firstProjectID,
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        productionEnvironmentID,
		Name:      "production",
		ProjectId: secondProjectID,
	}))
	resolver := newTestResolver(t, server)

	scope, err := resolver.ResolveScope(context.Background(), ScopeInput{
		EnvironmentIDOrName: stringPtr("production"),
	})

	require.NoError(t, err)
	assert.Equal(t, "tea-owner", scope.WorkspaceID)
	require.NotNil(t, scope.Project)
	assert.Equal(t, secondProjectID, scope.Project.Id)
	require.NotNil(t, scope.Environment)
	assert.Equal(t, productionEnvironmentID, scope.Environment.Id)
}

func TestResolverResolveScope_WorkspaceProjectConflictErrors(t *testing.T) {
	server := renderapi.NewServer(t)
	requestedWorkspaceID := testids.WorkspaceID("requested")
	projectID := testids.ProjectID("project")
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: requestedWorkspaceID, Name: "Requested Workspace"}))
	seedOwner(server, "tea-actual")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "My Project",
		OwnerId: "tea-actual",
	}))
	resolver := newTestResolver(t, server)

	_, err := resolver.ResolveScope(context.Background(), ScopeInput{
		WorkspaceIDOrName: requestedWorkspaceID,
		ProjectIDOrName:   stringPtr(projectID),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "belongs to workspace")
}

func TestResolverResolveScope_ProjectEnvironmentConflictErrors(t *testing.T) {
	server := renderapi.NewServer(t)
	firstProjectID := testids.ProjectID("first")
	secondProjectID := testids.ProjectID("second")
	environmentID := testids.EnvironmentID("production")
	seedOwner(server, "tea-owner")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      firstProjectID,
		Name:    "First Project",
		OwnerId: "tea-owner",
	}))
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      secondProjectID,
		Name:    "Second Project",
		OwnerId: "tea-owner",
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        environmentID,
		Name:      "production",
		ProjectId: secondProjectID,
	}))
	resolver := newTestResolver(t, server)

	_, err := resolver.ResolveScope(context.Background(), ScopeInput{
		ProjectIDOrName:     stringPtr(firstProjectID),
		EnvironmentIDOrName: stringPtr(environmentID),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "belongs to project")
}

func TestResolverResolveScope_AmbiguousEnvironmentNameErrors(t *testing.T) {
	server := renderapi.NewServer(t)
	firstProjectID := testids.ProjectID("first")
	secondProjectID := testids.ProjectID("second")
	seedOwner(server, "tea-owner")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      firstProjectID,
		Name:    "First Project",
		OwnerId: "tea-owner",
	}))
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      secondProjectID,
		Name:    "Second Project",
		OwnerId: "tea-owner",
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        testids.EnvironmentID("first"),
		Name:      "production",
		ProjectId: firstProjectID,
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        testids.EnvironmentID("second"),
		Name:      "production",
		ProjectId: secondProjectID,
	}))
	resolver := newTestResolver(t, server)

	_, err := resolver.ResolveScope(context.Background(), ScopeInput{
		EnvironmentIDOrName: stringPtr("production"),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple environments")
}

func TestResolverResolveEnvironment_ByID_ReturnsFullEnvironment(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("project")
	environmentID := testids.EnvironmentID("production")
	seedOwner(server, "tea-project-owner")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "My Project",
		OwnerId: "tea-project-owner",
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        environmentID,
		Name:      "production",
		ProjectId: projectID,
	}))
	resolver := newTestResolver(t, server)

	env, err := resolver.ResolveEnvironment(context.Background(), environmentID)

	require.NoError(t, err)
	require.NotNil(t, env)
	assert.Equal(t, environmentID, env.Id)
	assert.Equal(t, "production", env.Name)
	assert.Equal(t, projectID, env.ProjectId)
}

func TestResolverResolveEnvironment_PropagatesResolveScopeErrors(t *testing.T) {
	// Defers to ResolveScope; a missing environment surfaces the same error.
	server := renderapi.NewServer(t)
	seedOwner(server, "tea-project-owner")
	resolver := newTestResolver(t, server)

	env, err := resolver.ResolveEnvironment(context.Background(), "does-not-exist")

	require.Error(t, err)
	assert.Nil(t, env)
}

func TestResolverResolveScopeInActiveWorkspace_NarrowsProjectAndEnvironmentNames(t *testing.T) {
	t.Setenv("RENDER_WORKSPACE", "tea-active")

	server := renderapi.NewServer(t)
	seedOwner(server, "tea-active")
	seedOwner(server, "tea-other")
	activeProjectID := testids.ProjectID("active")
	otherProjectID := testids.ProjectID("other")
	activeEnvironmentID := testids.EnvironmentID("active")
	otherEnvironmentID := testids.EnvironmentID("other")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      activeProjectID,
		Name:    "Duplicated Name",
		OwnerId: "tea-active",
	}))
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      otherProjectID,
		Name:    "Duplicated Name",
		OwnerId: "tea-other",
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        activeEnvironmentID,
		Name:      "production",
		ProjectId: activeProjectID,
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        otherEnvironmentID,
		Name:      "production",
		ProjectId: otherProjectID,
	}))
	resolver := newTestResolver(t, server)

	scope, err := resolver.ResolveScopeInActiveWorkspace(context.Background(), ActiveWorkspaceScopeInput{
		ProjectIDOrName:     stringPtr("Duplicated Name"),
		EnvironmentIDOrName: stringPtr("production"),
	})

	require.NoError(t, err)
	assert.Equal(t, "tea-active", scope.WorkspaceID)
	require.NotNil(t, scope.Project)
	assert.Equal(t, activeProjectID, scope.Project.Id)
	require.NotNil(t, scope.Environment)
	assert.Equal(t, activeEnvironmentID, scope.Environment.Id)
}

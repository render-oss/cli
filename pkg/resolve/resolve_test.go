package resolve

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/project"
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

// Accepted project and environment names can match the ID shape
// (prefix plus 20 lowercase alphanumerics). These must still resolve by name
// when no object has that ID.
const (
	idShapedProjectName     = "prj-00000000000000093206"
	idShapedEnvironmentName = "evm-00000000000000093206"
)

func TestResolverResolveProject_IDShapedNameFallsBackToNameOnNotFound(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("id shaped name")
	seedOwner(server, "tea-owner")
	seedOwner(server, "tea-other")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    idShapedProjectName,
		OwnerId: "tea-owner",
	}))
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      testids.ProjectID("other workspace"),
		Name:    idShapedProjectName,
		OwnerId: "tea-other",
	}))
	resolver := newTestResolver(t, server)

	proj, err := resolver.ResolveProject(context.Background(), idShapedProjectName, "tea-owner")

	require.NoError(t, err)
	assert.Equal(t, projectID, proj.Id)
	assert.True(t, server.HasRequest("GET", "/projects/"+idShapedProjectName))
	assert.True(t, server.HasRequest("GET", "ownerId=tea-owner"))
}

func TestResolverResolveProject_RealIDTakesPrecedenceOverName(t *testing.T) {
	server := renderapi.NewServer(t)
	byIDProjectID := testids.ProjectID("by id")
	seedOwner(server, "tea-owner")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      byIDProjectID,
		Name:    "By ID",
		OwnerId: "tea-owner",
	}))
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      testids.ProjectID("by name"),
		Name:    byIDProjectID,
		OwnerId: "tea-owner",
	}))
	resolver := newTestResolver(t, server)

	proj, err := resolver.ResolveProject(context.Background(), byIDProjectID, "")

	require.NoError(t, err)
	assert.Equal(t, byIDProjectID, proj.Id)
	assert.False(t, server.HasRequest("GET", "name="))
}

func TestResolverResolveProject_MissingIDWithoutNameMatchKeepsNotFoundError(t *testing.T) {
	server := renderapi.NewServer(t)
	seedOwner(server, "tea-owner")
	resolver := newTestResolver(t, server)

	_, err := resolver.ResolveProject(context.Background(), idShapedProjectName, "tea-owner")

	require.EqualError(t, err, `project "prj-00000000000000093206" not found: unexpected response: 404 Not Found`)
	assert.True(t, errors.Is(err, project.ErrProjectNotFound))
}

func TestResolverResolveProject_AmbiguousIDShapedNameErrors(t *testing.T) {
	server := renderapi.NewServer(t)
	seedOwner(server, "tea-owner")
	for _, label := range []string{"first", "second"} {
		server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
			Id:      testids.ProjectID(label),
			Name:    idShapedProjectName,
			OwnerId: "tea-owner",
		}))
	}
	resolver := newTestResolver(t, server)

	_, err := resolver.ResolveProject(context.Background(), idShapedProjectName, "tea-owner")

	require.ErrorContains(t, err, "multiple projects found")
}

func TestResolverResolveScope_IDShapedEnvironmentNameFallsBackToNameOnNotFound(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("env id shaped project")
	otherProjectID := testids.ProjectID("env id shaped other")
	environmentID := testids.EnvironmentID("env id shaped")
	seedOwner(server, "tea-owner")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    idShapedProjectName,
		OwnerId: "tea-owner",
	}))
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      otherProjectID,
		Name:    "Other Project",
		OwnerId: "tea-owner",
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        environmentID,
		Name:      idShapedEnvironmentName,
		ProjectId: projectID,
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        testids.EnvironmentID("env id shaped other"),
		Name:      idShapedEnvironmentName,
		ProjectId: otherProjectID,
	}))
	resolver := newTestResolver(t, server)

	scope, err := resolver.ResolveScope(context.Background(), ScopeInput{
		ProjectIDOrName:     stringPtr(idShapedProjectName),
		EnvironmentIDOrName: stringPtr(idShapedEnvironmentName),
	})

	require.NoError(t, err)
	require.NotNil(t, scope.Project)
	assert.Equal(t, projectID, scope.Project.Id)
	require.NotNil(t, scope.Environment)
	assert.Equal(t, environmentID, scope.Environment.Id)
	assert.True(t, server.HasRequest("GET", "/environments/"+idShapedEnvironmentName))
}

func TestResolverResolveScope_AmbiguousIDShapedEnvironmentNameErrors(t *testing.T) {
	server := renderapi.NewServer(t)
	seedOwner(server, "tea-owner")
	for _, label := range []string{"first", "second"} {
		projectID := testids.ProjectID(label)
		server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
			Id:      projectID,
			Name:    label,
			OwnerId: "tea-owner",
		}))
		server.Environments.Add(renderapi.NewEnvironment(client.Environment{
			Id:        testids.EnvironmentID(label),
			Name:      idShapedEnvironmentName,
			ProjectId: projectID,
		}))
	}
	resolver := newTestResolver(t, server)

	_, err := resolver.ResolveScope(context.Background(), ScopeInput{
		WorkspaceIDOrName:   "tea-owner",
		EnvironmentIDOrName: stringPtr(idShapedEnvironmentName),
	})

	require.ErrorContains(t, err, "multiple environments found")
}

func TestResolverResolveScope_MissingEnvironmentIDWithoutNameMatchKeepsNotFoundError(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("missing env project")
	seedOwner(server, "tea-owner")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "Project",
		OwnerId: "tea-owner",
	}))
	resolver := newTestResolver(t, server)

	_, err := resolver.ResolveScope(context.Background(), ScopeInput{
		ProjectIDOrName:     stringPtr(projectID),
		EnvironmentIDOrName: stringPtr(idShapedEnvironmentName),
	})

	require.EqualError(t, err, `environment "evm-00000000000000093206" not found: unexpected response: 404 Not Found`)
	assert.True(t, errors.Is(err, environment.ErrEnvironmentNotFound))
}

func TestResolverResolveScope_IDShapedEnvironmentNameStaysScopedToProject(t *testing.T) {
	server := renderapi.NewServer(t)
	projectID := testids.ProjectID("scoped project")
	otherProjectID := testids.ProjectID("scoped other")
	seedOwner(server, "tea-owner")
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      projectID,
		Name:    "Project",
		OwnerId: "tea-owner",
	}))
	server.Projects.Add(renderapi.NewProject(renderapi.ProjectAttrs{
		Id:      otherProjectID,
		Name:    "Other",
		OwnerId: "tea-owner",
	}))
	server.Environments.Add(renderapi.NewEnvironment(client.Environment{
		Id:        testids.EnvironmentID("scoped other"),
		Name:      idShapedEnvironmentName,
		ProjectId: otherProjectID,
	}))
	resolver := newTestResolver(t, server)

	_, err := resolver.ResolveScope(context.Background(), ScopeInput{
		ProjectIDOrName:     stringPtr(projectID),
		EnvironmentIDOrName: stringPtr(idShapedEnvironmentName),
	})

	require.ErrorIs(t, err, environment.ErrEnvironmentNotFound)
	assert.True(t, server.HasRequest("GET", "projectId="+projectID))
}

// statusServer answers every detail GET with a fixed response and records
// whether any list (name lookup) request was made.
type statusServer struct {
	mu          sync.Mutex
	listCalls   int
	status      int
	contentType string
	body        string
}

func newStatusServer(t *testing.T, status int, contentType, body string) (*statusServer, *Resolver) {
	t.Helper()
	ss := &statusServer{status: status, contentType: contentType, body: body}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/projects" || r.URL.Path == "/environments" {
			ss.mu.Lock()
			ss.listCalls++
			ss.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/projects/") || strings.HasPrefix(r.URL.Path, "/environments/") {
			if ss.contentType != "" {
				w.Header().Set("Content-Type", ss.contentType)
			}
			w.WriteHeader(ss.status)
			_, _ = w.Write([]byte(ss.body))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	c, err := client.NewClientWithResponses(srv.URL)
	require.NoError(t, err)
	return ss, NewFromClient(c)
}

func TestResolver_IDShapedSelectorsOnlyFallBackToNameOn404(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		contentType string
		body        string
		wantErr     string
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, contentType: "application/json", body: `{"message":"unauthorized"}`, wantErr: "401 Unauthorized"},
		{name: "forbidden", status: http.StatusForbidden, contentType: "application/json", body: `{"message":"forbidden"}`, wantErr: "403 Forbidden"},
		{name: "rate limited", status: http.StatusTooManyRequests, contentType: "application/json", body: `{"message":"slow down"}`, wantErr: "429 Too Many Requests"},
		{name: "server error", status: http.StatusInternalServerError, contentType: "application/json", body: `{"message":"boom"}`, wantErr: "500 Internal Server Error"},
		{name: "200 without JSON body", status: http.StatusOK, contentType: "text/plain", body: "ok", wantErr: "unexpected response: 200 OK"},
		{name: "200 with malformed JSON", status: http.StatusOK, contentType: "application/json", body: "{", wantErr: "unexpected end of JSON input"},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/project", func(t *testing.T) {
			ss, resolver := newStatusServer(t, tc.status, tc.contentType, tc.body)

			_, err := resolver.ResolveProject(context.Background(), idShapedProjectName, "tea-owner")

			require.ErrorContains(t, err, tc.wantErr)
			assert.False(t, errors.Is(err, project.ErrProjectNotFound))
			assert.Zero(t, ss.listCalls, "non-404 project detail error must not fall back to name lookup")
		})
		t.Run(tc.name+"/environment", func(t *testing.T) {
			ss, resolver := newStatusServer(t, tc.status, tc.contentType, tc.body)

			_, err := resolver.ResolveScope(context.Background(), ScopeInput{
				EnvironmentIDOrName: stringPtr(idShapedEnvironmentName),
			})

			require.ErrorContains(t, err, tc.wantErr)
			assert.False(t, errors.Is(err, environment.ErrEnvironmentNotFound))
			assert.Zero(t, ss.listCalls, "non-404 environment detail error must not fall back to name lookup")
		})
	}

	t.Run("transport error", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		url := srv.URL
		srv.Close()
		c, err := client.NewClientWithResponses(url)
		require.NoError(t, err)
		resolver := NewFromClient(c)

		_, err = resolver.ResolveProject(context.Background(), idShapedProjectName, "tea-owner")
		require.Error(t, err)
		assert.False(t, errors.Is(err, project.ErrProjectNotFound))

		_, err = resolver.ResolveScope(context.Background(), ScopeInput{
			EnvironmentIDOrName: stringPtr(idShapedEnvironmentName),
		})
		require.Error(t, err)
		assert.False(t, errors.Is(err, environment.ErrEnvironmentNotFound))
	})
}

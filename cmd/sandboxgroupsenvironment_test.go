package cmd

import (
	"net/http"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

var (
	sandboxGroupsProductionEnvID  = testids.EnvironmentID("production")
	sandboxGroupsStagingEnvID     = testids.EnvironmentID("staging")
	sandboxGroupsOtherEnvID       = testids.EnvironmentID("other-workspace")
	sandboxGroupsOtherWorkspaceID = testids.WorkspaceID("other")
	sandboxGroupsUnboundGroupID   = testids.SandboxGroupID("unbound")
	sandboxGroupsStagingGroupID   = testids.SandboxGroupID("staging")
	sandboxGroupsUnknownGroupID   = testids.SandboxGroupID("unknown")
)

func seedSandboxGroupEnvironments(server *renderapi.Server) {
	server.CreateProject(
		renderapi.ProjectAttrs{
			Name:    "My Project",
			OwnerId: sandboxGroupsActiveWorkspaceID,
		},
		renderapi.EnvAttrs{
			ID:   sandboxGroupsProductionEnvID,
			Name: "production",
		},
		renderapi.EnvAttrs{
			ID:   sandboxGroupsStagingEnvID,
			Name: "staging",
		},
	)
	server.Owners.Add(renderapi.NewOwner(client.Owner{
		Id:   sandboxGroupsOtherWorkspaceID,
		Name: "Other Workspace",
	}))
	server.CreateProject(
		renderapi.ProjectAttrs{
			Name:    "Other Project",
			OwnerId: sandboxGroupsOtherWorkspaceID,
		},
		renderapi.EnvAttrs{
			ID:   sandboxGroupsOtherEnvID,
			Name: "other",
		},
	)
}

func jsonStringField(fields map[string]any, key string) string {
	value, _ := fields[key].(string)
	return value
}

func newSandboxGroupsEnvironmentServer(t *testing.T) *renderapi.Server {
	t.Helper()

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{
		Id:   sandboxGroupsActiveWorkspaceID,
		Name: "Test Workspace",
	}))
	t.Setenv("RENDER_WORKSPACE", sandboxGroupsActiveWorkspaceID)
	seedSandboxGroupEnvironments(server)

	server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{
		Id:      sandboxGroupsUnboundGroupID,
		Name:    "unbound",
		OwnerId: sandboxGroupsActiveWorkspaceID,
	}))
	server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{
		Id:            sandboxGroupsStagingGroupID,
		Name:          "in-staging",
		OwnerId:       sandboxGroupsActiveWorkspaceID,
		EnvironmentId: new(sandboxGroupsStagingEnvID),
	}))

	return server
}

type sandboxGroupsEnvironmentRequest struct {
	Method string
	URI    string
	Body   map[string]any
}

type sandboxGroupsEnvironmentOutcome struct {
	Err                 string
	Requests            []sandboxGroupsEnvironmentRequest
	OutputEnvironmentID string
}

func sandboxGroupsEnvironmentOutcomeOf(
	t *testing.T,
	server *renderapi.Server,
	result CommandResult,
	err error,
) sandboxGroupsEnvironmentOutcome {
	t.Helper()

	got := sandboxGroupsEnvironmentOutcome{}
	if err != nil {
		got.Err = err.Error()
	}

	for _, req := range server.Requests {
		got.Requests = appendSandboxGroupsEnvironmentRequest(t, got.Requests, req)
	}

	if err == nil {
		output := unmarshalSandboxGroupJSONOutput(t, result.Stdout)
		got.OutputEnvironmentID = jsonStringField(output, "environmentId")
	}

	return got
}

func appendSandboxGroupsEnvironmentRequest(
	t *testing.T,
	requests []sandboxGroupsEnvironmentRequest,
	req renderapi.RecordedRequest,
) []sandboxGroupsEnvironmentRequest {
	t.Helper()

	if req.Method == http.MethodGet {
		return requests
	}

	sent := sandboxGroupsEnvironmentRequest{
		Method: req.Method,
		URI:    req.URI,
	}
	if len(req.Body) > 0 {
		sent.Body = testrequire.ParseJSONMap(t, string(req.Body))
	}

	return append(requests, sent)
}

func environmentResourcesURI(environmentID string) string {
	return "/environments/" + environmentID + "/resources"
}

func resourceIDsBody(resourceID string) map[string]any {
	return map[string]any{
		"resourceIds": []any{resourceID},
	}
}

package sandboximage_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/sandboxgroup"
	"github.com/render-oss/cli/pkg/sandboximage"
)

const testWorkspace = "tea-workspace"

type recordedRequest struct {
	Method string
	Path   string
	Query  map[string][]string
	Body   map[string]any
}

func newTestService(t *testing.T, groups []client.SandboxGroupWithCursor, respond func(w http.ResponseWriter)) (*sandboximage.Service, *[]recordedRequest) {
	t.Helper()
	var recorded []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query()}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&rec.Body)
		}
		recorded = append(recorded, rec)
		if r.URL.Path == "/sandbox-groups" {
			respondJSON(http.StatusOK, groups)(w)
			return
		}
		respond(w)
	}))
	t.Cleanup(server.Close)
	t.Setenv("RENDER_WORKSPACE", testWorkspace)

	c, err := client.NewClientWithResponses(server.URL)
	require.NoError(t, err)
	return sandboximage.NewService(sandboximage.NewRepo(c), sandboxgroup.NewService(sandboxgroup.NewRepo(c))), &recorded
}

func respondJSON(status int, v any) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
}

func defaultGroup(id string) []client.SandboxGroupWithCursor {
	return []client.SandboxGroupWithCursor{{Cursor: "c0", SandboxGroup: sandboxesclient.SandboxGroup{Id: id, IsDefault: true}}}
}

func TestServiceImport_Body(t *testing.T) {
	tests := []struct {
		name           string
		input          sandboximage.ImportInput
		wantCredential map[string]any
	}{
		{
			name:  "no credential omits registryCredential",
			input: sandboximage.ImportInput{SandboxGroupID: "sbg-1", Name: "python", Source: "python:3.13"},
		},
		{
			name:           "credential is sent",
			input:          sandboximage.ImportInput{SandboxGroupID: "sbg-1", Name: "agent", Source: "ghcr.io/acme/agent:1.0", RegistryUsername: "bot", RegistryPassword: "s3cret"},
			wantCredential: map[string]any{"username": "bot", "password": "s3cret"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, recorded := newTestService(t, nil, respondJSON(http.StatusCreated, sandboxesclient.SandboxBuild{
				Id: "sbb-1", Status: sandboxesclient.SandboxBuildStatusRunning,
			}))

			image, err := svc.Import(context.Background(), tc.input)
			require.NoError(t, err)
			assert.Equal(t, "sbb-1", image.Id)

			require.Len(t, *recorded, 1, "an explicit group needs no lookup")
			rec := (*recorded)[0]
			assert.Equal(t, http.MethodPost, rec.Method)
			assert.Equal(t, "/sandbox-groups/sbg-1/builds", rec.Path)
			assert.NotContains(t, rec.Query, "ownerId")
			assert.Equal(t, tc.input.Name, rec.Body["name"])
			assert.Equal(t, tc.input.Source, rec.Body["sourceImage"])
			if tc.wantCredential == nil {
				assert.NotContains(t, rec.Body, "registryCredential")
			} else {
				assert.Equal(t, tc.wantCredential, rec.Body["registryCredential"])
			}
		})
	}
}

func TestServiceImport_ResolvesDefaultGroup(t *testing.T) {
	svc, recorded := newTestService(t, defaultGroup("sbg-default"), respondJSON(http.StatusCreated, sandboxesclient.SandboxBuild{Id: "sbb-1"}))

	_, err := svc.Import(context.Background(), sandboximage.ImportInput{Name: "python", Source: "python:3.13"})
	require.NoError(t, err)

	require.Len(t, *recorded, 2)
	assert.Equal(t, "/sandbox-groups", (*recorded)[0].Path)
	assert.Equal(t, []string{testWorkspace}, (*recorded)[0].Query["ownerId"])
	assert.Equal(t, "/sandbox-groups/sbg-default/builds", (*recorded)[1].Path)
}

func TestServiceGet_SendsGroupWithoutOwner(t *testing.T) {
	svc, recorded := newTestService(t, nil, respondJSON(http.StatusOK, sandboxesclient.SandboxBuild{Id: "sbb-1"}))

	image, err := svc.Get(context.Background(), "sbg-1", "sbb-1")
	require.NoError(t, err)
	assert.Equal(t, "sbb-1", image.Id)

	require.Len(t, *recorded, 1)
	assert.Equal(t, http.MethodGet, (*recorded)[0].Method)
	assert.Equal(t, "/sandbox-groups/sbg-1/builds/sbb-1", (*recorded)[0].Path)
	assert.NotContains(t, (*recorded)[0].Query, "ownerId")
}

func TestServiceGet_SurfacesNotFound(t *testing.T) {
	message := "not found: build"
	svc, _ := newTestService(t, nil, respondJSON(http.StatusNotFound, client.Error{Message: &message}))

	_, err := svc.Get(context.Background(), "sbg-1", "sbb-missing")
	require.ErrorContains(t, err, "404")
	assert.ErrorContains(t, err, "not found: build")
}

func TestService_NoDefaultGroupReturnsError(t *testing.T) {
	svc, recorded := newTestService(t, []client.SandboxGroupWithCursor{}, respondJSON(http.StatusOK, sandboxesclient.SandboxBuild{}))

	_, err := svc.Get(context.Background(), "", "sbb-1")
	require.ErrorContains(t, err, "no default sandbox group")
	assert.Len(t, *recorded, 1, "only the group lookup runs")
}

func TestService_MissingWorkspaceReturnsError(t *testing.T) {
	svc, recorded := newTestService(t, nil, respondJSON(http.StatusCreated, sandboxesclient.SandboxBuild{}))
	t.Setenv("RENDER_WORKSPACE", "")
	t.Setenv("RENDER_CLI_CONFIG_PATH", t.TempDir()+"/nonexistent.yaml")

	_, err := svc.Import(context.Background(), sandboximage.ImportInput{Name: "python", Source: "python:3.13"})
	require.Error(t, err)
	assert.Empty(t, *recorded, "no request should be sent without a workspace")
}

package sandboxsnapshot_test

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
	"github.com/render-oss/cli/pkg/sandboxsnapshot"
)

const testWorkspace = "tea-workspace"

type recordedRequest struct {
	Method string
	Path   string
	Query  map[string][]string
	Body   map[string]any
}

func newTestService(t *testing.T, respond func(w http.ResponseWriter)) (*sandboxsnapshot.Service, *recordedRequest) {
	t.Helper()
	rec := &recordedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.Method = r.Method
		rec.Path = r.URL.Path
		rec.Query = r.URL.Query()
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&rec.Body)
		}
		respond(w)
	}))
	t.Cleanup(server.Close)
	t.Setenv("RENDER_WORKSPACE", testWorkspace)

	c, err := client.NewClientWithResponses(server.URL)
	require.NoError(t, err)
	return sandboxsnapshot.NewService(sandboxsnapshot.NewRepo(c), sandboxgroup.NewRepo(c)), rec
}

func respondJSON(status int, v any) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
}

func TestServiceCreate_KindBody(t *testing.T) {
	tests := []struct {
		name       string
		kind       string
		wantKind   string
		wantAbsent bool
	}{
		{name: "unset omits kind so the server default applies", wantAbsent: true},
		{name: "filesystem is sent explicitly", kind: "filesystem", wantKind: "filesystem"},
		{name: "runtime is sent explicitly", kind: "runtime", wantKind: "runtime"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, rec := newTestService(t, respondJSON(http.StatusAccepted, sandboxesclient.SandboxSnapshot{
				Id: "snp-1", Status: sandboxesclient.SandboxSnapshotStatusCreating,
			}))

			snap, err := svc.Create(context.Background(), sandboxsnapshot.CreateInput{SandboxID: "sbx-1", Kind: tc.kind})
			require.NoError(t, err)
			assert.Equal(t, "snp-1", snap.Id)
			assert.Equal(t, sandboxesclient.SandboxSnapshotStatusCreating, snap.Status)

			assert.Equal(t, http.MethodPost, rec.Method)
			assert.Equal(t, "/sandboxes/sbx-1/snapshots", rec.Path)
			assert.Equal(t, []string{testWorkspace}, rec.Query["ownerId"])
			kind, present := rec.Body["kind"]
			if tc.wantAbsent {
				assert.False(t, present, "kind should be omitted, got %v", kind)
				return
			}
			assert.Equal(t, tc.wantKind, kind)
		})
	}
}

func TestServiceGet_SendsGroupAndOwner(t *testing.T) {
	svc, rec := newTestService(t, respondJSON(http.StatusOK, sandboxesclient.SandboxSnapshot{Id: "snp-1"}))

	got, err := svc.Get(context.Background(), "sbg-1", "snp-1")
	require.NoError(t, err)
	assert.Equal(t, "snp-1", got.Id)
	assert.Equal(t, http.MethodGet, rec.Method)
	assert.Equal(t, "/sandbox-groups/sbg-1/snapshots/snp-1", rec.Path)
	assert.Equal(t, []string{testWorkspace}, rec.Query["ownerId"])
}

func TestService_MissingWorkspaceReturnsError(t *testing.T) {
	svc, rec := newTestService(t, respondJSON(http.StatusAccepted, sandboxesclient.SandboxSnapshot{}))
	t.Setenv("RENDER_WORKSPACE", "")
	t.Setenv("RENDER_CLI_CONFIG_PATH", t.TempDir()+"/nonexistent.yaml")

	_, err := svc.Create(context.Background(), sandboxsnapshot.CreateInput{SandboxID: "sbx-1"})
	require.Error(t, err)
	assert.Empty(t, rec.Method, "no request should be sent without a workspace")
}

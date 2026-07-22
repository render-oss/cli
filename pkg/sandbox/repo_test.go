package sandbox

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/client"
	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRepo builds a Repo whose generated client targets baseURL and signs
// requests with a bearer apiKey, matching how the DI container wires the client.
func newTestRepo(t *testing.T, baseURL, apiKey string) *Repo {
	t.Helper()
	apiClient, err := client.NewClientWithResponses(baseURL, client.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		return nil
	}))
	require.NoError(t, err)
	return NewRepo(apiClient)
}

func TestExecSandboxStream(t *testing.T) {
	const (
		workspace = "tea-workspace"
		apiKey    = "api-key-xyz"
		runToken  = "run-token-123"
		sandboxID = "sbx-abc123"
	)

	var (
		mintAuth    string
		mintOwnerID string
		streamAuth  string
		streamBody  string
	)

	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/runs/stream/token", func(w http.ResponseWriter, r *http.Request) {
		mintAuth = r.Header.Get("Authorization")
		mintOwnerID = r.URL.Query().Get("ownerId")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(sandboxclient.SandboxConnectResponse{
			ExecutionId: "exec-1",
			Token:       runToken,
			Uri:         serverURL + "/exec/stream",
			Method:      http.MethodPost,
			ExpiresAt:   time.Now().Add(time.Hour),
		})
	})
	mux.HandleFunc("/exec/stream", func(w http.ResponseWriter, r *http.Request) {
		streamAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		streamBody = string(b)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: output\ndata: {\"stream\":\"stdout\",\"data\":\"hello\"}\n\nevent: exit\ndata: {\"exit_code\":0}\n\n")
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	serverURL = srv.URL

	t.Setenv("RENDER_WORKSPACE", workspace)

	repo := newTestRepo(t, srv.URL+"/v1/", apiKey)

	var outputs []ExecOutputEvent
	exitCode, err := repo.ExecSandboxStream(context.Background(), sandboxID, "echo hello", func(e *ExecOutputEvent) error {
		outputs = append(outputs, *e)
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "Bearer "+apiKey, mintAuth)
	assert.Equal(t, workspace, mintOwnerID)
	assert.Equal(t, "Bearer "+runToken, streamAuth)
	assert.JSONEq(t, `{"command":"echo hello"}`, streamBody)
	require.Len(t, outputs, 1)
	assert.Equal(t, ExecOutputStreamStdout, outputs[0].Stream)
	assert.Equal(t, "hello", outputs[0].Data)
}

func TestExecSandboxStreamMintError(t *testing.T) {
	const sandboxID = "sbx-err"

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/runs/stream/token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "sandbox not found"})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Setenv("RENDER_WORKSPACE", "tea-workspace")

	repo := newTestRepo(t, srv.URL+"/v1/", "api-key-xyz")

	_, err := repo.ExecSandboxStream(context.Background(), sandboxID, "echo hello", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sandbox not found")
}

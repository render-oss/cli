package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/pointers"
)

const (
	restartTestPostgresID = "dpg-00000000000000000000"
	restartTestServiceID  = "srv-00000000000000000000"
)

// executeRestart runs `render restart` against a server that answers every
// POST /{postgres,services}/{id}/restart with status (and a NOT_FOUND body for
// error statuses). It returns the command output, restart paths hit and error.
func executeRestart(t *testing.T, status int, args ...string) (CommandResult, []string, error) {
	t.Helper()

	var restartPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method, "unexpected request %s %s", r.Method, r.URL.Path)
		restartPaths = append(restartPaths, r.URL.Path)
		if status < http.StatusBadRequest {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(client.Error{
			Code:    pointers.From("NOT_FOUND"),
			Id:      pointers.From("not_found"),
			Message: pointers.From("not found"),
		})
	}))
	t.Cleanup(server.Close)

	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_HOST", server.URL)
	t.Setenv("RENDER_API_KEY", "test-api-key")

	c, err := client.NewClientWithResponses(server.URL)
	require.NoError(t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{}, nil
	}

	root := newRootCmd()
	cmd := *restartCmd
	cmd.ResetFlags()
	root.AddCommand(&cmd)
	setupRootCmdPersistentRun(root, deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(append([]string{"restart"}, args...))

	execErr := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, restartPaths, execErr
}

func TestRestart_PostgresAPIRefusal_FailsInEveryOutputFormat(t *testing.T) {
	for _, output := range []string{"json", "yaml", "text"} {
		t.Run(output, func(t *testing.T) {
			result, paths, err := executeRestart(t, http.StatusNotFound, restartTestPostgresID, "--confirm", "-o", output)

			require.EqualError(t, err, "failed to restart resource: received response code 404 (NOT_FOUND): not found")
			assert.Equal(t, []string{"/postgres/" + restartTestPostgresID + "/restart"}, paths)
			assert.Empty(t, result.Stdout)
			assert.NotContains(t, result.Stdout, "restarted successfully")
			assert.Contains(t, result.Stderr, "received response code 404 (NOT_FOUND): not found")
		})
	}
}

func TestRestart_ServiceAPIRefusal_Fails(t *testing.T) {
	result, paths, err := executeRestart(t, http.StatusNotFound, restartTestServiceID, "--confirm", "-o", "json")

	require.EqualError(t, err, "failed to restart resource: received response code 404 (NOT_FOUND): not found")
	assert.Equal(t, []string{"/services/" + restartTestServiceID + "/restart"}, paths)
	assert.Empty(t, result.Stdout)
}

func TestRestart_PostgresAccepted_ReportsSuccess(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusAccepted} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			result, paths, err := executeRestart(t, status, restartTestPostgresID, "--confirm", "-o", "json")

			require.NoError(t, err)
			assert.Equal(t, []string{"/postgres/" + restartTestPostgresID + "/restart"}, paths)
			assert.JSONEq(t, `"`+restartTestPostgresID+` restarted successfully"`, result.Stdout)
		})
	}
}

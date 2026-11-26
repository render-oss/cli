package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func executeEnvGroup(t *testing.T, server *httptest.Server, args ...string) (string, error) {
	t.Helper()
	c, err := client.NewClientWithResponses(server.URL)
	require.NoError(t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) { return command.RuntimeSignals{}, nil }
	root := newRootCmd()
	root.AddCommand(newEnvGroupCmd(deps))
	setupRootCmdPersistentRun(root, deps)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), err
}

func TestEnvGroupGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/env-groups/evg-test", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"evg-test","envVars":[{"key":"FOO","value":"hello=world"}]}`))
	}))
	defer server.Close()
	out, err := executeEnvGroup(t, server, "env-group", "get", "evg-test", "--output", "json")
	require.NoError(t, err)
	assert.Contains(t, out, `"key": "FOO"`)
	assert.Contains(t, out, `hello=world`)
}

func TestEnvGroupSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/env-groups/evg-test/env-vars/FOO", r.URL.Path)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "a=b", body["value"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"evg-test"}`))
	}))
	defer server.Close()
	out, err := executeEnvGroup(t, server, "env-group", "set", "evg-test", "FOO=a=b", "--output", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "FOO")
	assert.NotContains(t, out, "a=b")
}

func TestEnvGroupGetOmittedVariables(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"evg-test"}`))
	}))
	defer server.Close()
	_, err := executeEnvGroup(t, server, "env-group", "get", "evg-test")
	require.ErrorContains(t, err, "permission")
}

func TestEnvGroupSetInvalidInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { t.Error("unexpected API request") }))
	defer server.Close()
	for _, input := range []string{"FOO", "=value"} {
		_, err := executeEnvGroup(t, server, "env-group", "set", "evg-test", input)
		require.ErrorContains(t, err, "KEY=value")
	}
}

func TestEnvGroupSetUnexpectedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer server.Close()
	out, err := executeEnvGroup(t, server, "env-group", "set", "evg-test", "KEY=secret")
	require.ErrorContains(t, err, "500")
	assert.NotContains(t, out, "Updated KEY")
}

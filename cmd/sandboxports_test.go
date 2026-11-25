package cmd

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	sandboxes "github.com/render-oss/cli/pkg/client/sandboxes"
)

func newSandboxPortsServer(t *testing.T) *renderapi.Server {
	t.Helper()
	server := newSandboxCreateServer(t)
	server.Sandboxes.Add(renderapi.NewSandbox(sandboxes.Sandbox{Id: "sbx-run"}))
	return server
}

func executePortsCommand(t *testing.T, server *renderapi.Server, args ...string) (CommandResult, error) {
	t.Helper()
	return executeSandboxCommand(t, server, append([]string{"sandboxes", "ports"}, args...)...)
}

func assertActiveWorkspaceQuery(t *testing.T, server *renderapi.Server, method, path string) {
	t.Helper()
	req, ok := server.LastRequest(method, path)
	require.True(t, ok, "no %s %s request", method, path)
	u, err := url.Parse(req.URI)
	require.NoError(t, err)
	assert.Equal(t, sandboxCreateActiveWorkspaceID, u.Query().Get("ownerId"))
}

func TestSandboxPortsExpose(t *testing.T) {
	server := newSandboxPortsServer(t)

	result, err := executePortsCommand(t, server, "expose", "sbx-run", "8080", "--output", "text")
	require.NoError(t, err)

	assert.Equal(t, "Port: 8080\nURL:  https://sbx-run--8080.onrender.run\n", result.Stdout)
	assert.Equal(t, []int{8080}, portNumbers(server.Sandboxes.Ports("sbx-run")))
	assertActiveWorkspaceQuery(t, server, "PUT", "/sandboxes/sbx-run/ports/8080")
}

func TestSandboxPortsExpose_JSON(t *testing.T) {
	server := newSandboxPortsServer(t)

	result, err := executePortsCommand(t, server, "expose", "sbx-run", "8080", "--output", "json")
	require.NoError(t, err)

	var got sandboxes.SandboxExposedPort
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &got), result.Stdout)
	assert.Equal(t, sandboxes.SandboxExposedPort{
		Protocol: sandboxes.Tcp,
		Port:     8080,
		Url:      "https://sbx-run--8080.onrender.run",
	}, got)
}

func TestSandboxPortsExpose_Errors(t *testing.T) {
	tests := []struct {
		name    string
		seeded  []int
		args    []string
		wantErr string
	}{
		{
			name:    "port that is not a number",
			args:    []string{"sbx-run", "web"},
			wantErr: `invalid port "web"`,
		},
		{
			name:    "port out of range",
			args:    []string{"sbx-run", "0"},
			wantErr: "400",
		},
		{
			name:    "missing sandbox",
			args:    []string{"sbx-missing", "8080"},
			wantErr: "sandbox not found",
		},
		{
			name:    "port limit reached",
			seeded:  []int{3000, 3001, 3002, 3003, 3004, 3005, 3006, 3007, 3008, 3009},
			args:    []string{"sbx-run", "8080"},
			wantErr: "409 (port_limit_exceeded): sandbox can expose at most 10 ports",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newSandboxPortsServer(t)
			server.Sandboxes.ExposePorts("sbx-run", tc.seeded...)

			args := append([]string{"expose"}, tc.args...)
			result, err := executePortsCommand(t, server, append(args, "--output", "text")...)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.Empty(t, result.Stdout)
		})
	}
}

func TestSandboxPortsHide(t *testing.T) {
	tests := []struct {
		name   string
		seeded []int
	}{
		{name: "exposed port", seeded: []int{8080, 6080}},
		{name: "port that is not exposed", seeded: []int{6080}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newSandboxPortsServer(t)
			server.Sandboxes.ExposePorts("sbx-run", tc.seeded...)

			result, err := executePortsCommand(t, server, "hide", "sbx-run", "8080", "--output", "text")
			require.NoError(t, err)

			assert.Equal(t, "Port 8080 on sbx-run is not exposed\n", result.Stdout)
			assert.Equal(t, []int{6080}, portNumbers(server.Sandboxes.Ports("sbx-run")))
			assertActiveWorkspaceQuery(t, server, "DELETE", "/sandboxes/sbx-run/ports/8080")
		})
	}
}

func TestSandboxPortsHide_JSON(t *testing.T) {
	server := newSandboxPortsServer(t)
	server.Sandboxes.ExposePorts("sbx-run", 8080)

	result, err := executePortsCommand(t, server, "hide", "sbx-run", "8080", "--output", "json")
	require.NoError(t, err)

	assert.JSONEq(t, `{"sandboxId": "sbx-run", "port": 8080}`, result.Stdout)
}

func TestSandboxPortsList(t *testing.T) {
	server := newSandboxPortsServer(t)
	server.Sandboxes.ExposePorts("sbx-run", 8080, 6080)

	result, err := executePortsCommand(t, server, "list", "sbx-run", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "8080    https://sbx-run--8080.onrender.run")
	assert.Contains(t, result.Stdout, "6080    https://sbx-run--6080.onrender.run")
	assertActiveWorkspaceQuery(t, server, "GET", "/sandboxes/sbx-run/ports")
}

func TestSandboxPortsList_EmptyJSON(t *testing.T) {
	server := newSandboxPortsServer(t)

	result, err := executePortsCommand(t, server, "list", "sbx-run", "--output", "json")
	require.NoError(t, err)

	assert.JSONEq(t, `[]`, result.Stdout)
}

func TestSandboxCreate_ExposePorts(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "without wait"},
		{name: "with wait", args: []string{"--wait"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newSandboxCreateServer(t)
			args := append(
				[]string{"sandboxes", "create", "--expose-ports", "8080,6080", "--output", "json"},
				tc.args...,
			)

			result, err := executeSandboxCommand(t, server, args...)
			require.NoError(t, err)

			req, ok := server.LastRequest("POST", "/sandboxes")
			require.True(t, ok)
			var body struct {
				ExposedPorts []map[string]any `json:"exposed_ports"`
			}
			require.NoError(t, json.Unmarshal(req.Body, &body))
			assert.Equal(t, []map[string]any{
				{"protocol": "tcp", "port": float64(8080)},
				{"protocol": "tcp", "port": float64(6080)},
			}, body.ExposedPorts)

			var got struct {
				Id           string                         `json:"id"`
				Status       sandboxes.SandboxStatus        `json:"status"`
				ExposedPorts []sandboxes.SandboxExposedPort `json:"exposed_ports"`
			}
			require.NoError(t, json.Unmarshal([]byte(result.Stdout), &got), result.Stdout)
			assert.NotEmpty(t, got.Id)
			assert.Equal(t, []string{
				"https://" + got.Id + "--8080.onrender.run",
				"https://" + got.Id + "--6080.onrender.run",
			}, portURLs(got.ExposedPorts))
		})
	}
}

func TestSandboxCreate_ExposePortsText(t *testing.T) {
	server := newSandboxCreateServer(t)

	args := []string{"sandboxes", "create", "--expose-ports", "8080", "--output", "text"}
	result, err := executeSandboxCommand(t, server, args...)
	require.NoError(t, err)

	created := server.Sandboxes.Only(t)
	assert.Contains(t, result.Stdout, "Ports:   8080 https://"+created.Id+"--8080.onrender.run")
}

func TestSandboxCreate_WithoutExposePortsOmitsField(t *testing.T) {
	server := newSandboxCreateServer(t)

	result, err := executeSandboxCommand(t, server, "sandboxes", "create", "--output", "json")
	require.NoError(t, err)

	req, ok := server.LastRequest("POST", "/sandboxes")
	require.True(t, ok)
	assert.NotContains(t, string(req.Body), "exposed_ports")
	assert.NotContains(t, result.Stdout, "exposed_ports")
	assert.NotContains(t, result.Stdout, "Ports:")
}

func portNumbers(ports []sandboxes.SandboxExposedPort) []int {
	out := make([]int, len(ports))
	for i, p := range ports {
		out[i] = p.Port
	}
	return out
}

func portURLs(ports []sandboxes.SandboxExposedPort) []string {
	out := make([]string, len(ports))
	for i, p := range ports {
		out[i] = p.Url
	}
	return out
}

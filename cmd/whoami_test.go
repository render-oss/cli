package cmd

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testassert"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/dependencies"
)

// whoamiHarness runs whoami against a fake API and captures command output.
type whoamiHarness struct {
	server *renderapi.Server
	root   *cobra.Command
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// whoamiInitialConfig overrides the harness's default state.
// Zero-values are meaningful and will provide a standard happy path
type whoamiInitialConfig struct {
	// noAuthToken leaves the CLI without an API key or saved credentials when
	// true. The default false supplies a test API key.
	noAuthToken bool
	// getUsersHTTPErrorCode makes the next GET /users return this 4xx or 5xx
	// status. Zero leaves the endpoint behaving normally, with no queued error.
	getUsersHTTPErrorCode int
}

// newWhoamiHarness builds a Cobra app with whoami and seeds a single current
// user. It sets an API key and no active workspace by default.
func newWhoamiHarness(t *testing.T, initial ...whoamiInitialConfig) whoamiHarness {
	t.Helper()
	require.LessOrEqual(t, len(initial), 1)
	var initialConfig whoamiInitialConfig
	if len(initial) == 1 {
		initialConfig = initial[0]
	}
	server := renderapi.NewServer(t)
	server.SetCurrentUser(client.User{Id: "usr-d123456789abcdefghij", Name: "Jane Doe", Email: "email@example.com"})
	if status := initialConfig.getUsersHTTPErrorCode; status != 0 {
		require.GreaterOrEqual(t, status, http.StatusBadRequest, "GET /users error must be a 4xx or 5xx status")
		require.Less(t, status, 600, "GET /users error must be a 4xx or 5xx status")
		server.RespondToGetUsersWithError(status)
	}
	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_HOST", server.URL())
	apiKey := "test-api-key"
	if initialConfig.noAuthToken {
		apiKey = ""
	}
	t.Setenv("RENDER_API_KEY", apiKey)
	t.Setenv("RENDER_WORKSPACE", "")

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	deps := dependencies.New(c)
	root := newRootCmd()
	cmd := *whoamiCmd
	cmd.ResetFlags()
	root.AddCommand(&cmd)
	setupRootCmdPersistentRun(root, deps)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	return whoamiHarness{server: server, root: root, stdout: stdout, stderr: stderr}
}

// whoami executes the command with any additional arguments and captures output.
func (h whoamiHarness) whoami(args ...string) error {
	h.root.SetArgs(append([]string{"whoami"}, args...))
	return h.root.Execute()
}

func TestWhoami_NotLoggedIn(t *testing.T) {
	h := newWhoamiHarness(t, whoamiInitialConfig{noAuthToken: true})

	err := h.whoami()

	require.ErrorIs(t, err, config.ErrLogin)
	assert.Contains(t, h.stderr.String(), "run `render login` to authenticate")
	assert.Empty(t, h.stdout.String())
	assert.False(t, h.server.HasRequest("GET", "/users"))
}

func TestWhoami_APIUnauthorized(t *testing.T) {
	h := newWhoamiHarness(t, whoamiInitialConfig{getUsersHTTPErrorCode: http.StatusUnauthorized})

	err := h.whoami("-o", "json")

	require.ErrorIs(t, err, command.ErrTokenExpired)
	assert.Contains(t, h.stderr.String(), "your token is expired; run `render login` to get a new one")
	assert.Empty(t, h.stdout.String())
	assert.True(t, h.server.HasRequest("GET", "/users"))
}

func TestWhoami_HappyPath(t *testing.T) {
	t.Run("text and default", func(t *testing.T) {
		for _, tt := range []struct {
			name string
			args []string
		}{
			{name: "default"},
			{name: "explicit text", args: []string{"-o", "text"}},
		} {
			t.Run(tt.name, func(t *testing.T) {
				h := newWhoamiHarness(t)

				require.NoError(t, h.whoami(tt.args...))

				testassert.ContainsInOrder(t, h.stdout.String(),
					"Name:", "Jane Doe", "Email:", "email@example.com", "ID:", "usr-d123456789abcdefghij")
			})
		}
	})

	t.Run("json", func(t *testing.T) {
		h := newWhoamiHarness(t)

		require.NoError(t, h.whoami("-o", "json"))

		assert.JSONEq(t, `{"id":"usr-d123456789abcdefghij","name":"Jane Doe","email":"email@example.com"}`, h.stdout.String())
	})

	t.Run("yaml", func(t *testing.T) {
		h := newWhoamiHarness(t)

		require.NoError(t, h.whoami("-o", "yaml"))

		var got map[string]string
		require.NoError(t, yaml.Unmarshal(h.stdout.Bytes(), &got))
		assert.Equal(t, map[string]string{"id": "usr-d123456789abcdefghij", "name": "Jane Doe", "email": "email@example.com"}, got)
	})
}

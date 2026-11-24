package cmd

import (
	"bytes"
	"testing"

	"github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func executeWorkspacesCommand(t *testing.T, server *renderapi.Server, args ...string) (CommandResult, error) {
	t.Helper()
	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_HOST", server.URL())
	t.Setenv("RENDER_API_KEY", "test-api-key")
	t.Setenv("RENDER_WORKSPACE", "")

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{
			StdinTTY:  false,
			StdoutTTY: false,
			StderrTTY: false,
		}, nil
	}

	root := newRootCmd()
	root.AddCommand(workspacesCmd)
	setupRootCmdPersistentRun(root, deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)

	execErr := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, execErr
}

func TestWorkspaces_NonInteractive_ListsWorkspaces(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Name: "acme-corp", Email: "admin@acme.com"}))
	server.Owners.Add(renderapi.NewOwner(client.Owner{Name: "side-project", Email: "me@example.com"}))

	result, err := executeWorkspacesCommand(t, server, "workspaces", "--output", "text")

	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "acme-corp")
	assert.Contains(t, result.Stdout, "side-project")
	assert.True(t, server.HasRequest("GET", "/owners"), "expected GET /owners to be called")
}

func TestWorkspaces_NonInteractive_EmptyList(t *testing.T) {
	server := renderapi.NewServer(t)

	result, err := executeWorkspacesCommand(t, server, "workspaces", "--output", "text")

	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "NAME", "expected header row to be present")
	assert.NotContains(t, result.Stdout, "@", "expected no workspace rows — all workspace emails contain @")
	assert.True(t, server.HasRequest("GET", "/owners"), "expected GET /owners to be called")
}

// TestWorkspaces_ExplicitOutputOverridesInvalidRenderOutput runs the real root
// setup with the real runtime-signal detector, so RENDER_OUTPUT is read from the
// environment exactly as in a shipped binary.
func TestWorkspaces_ExplicitOutputOverridesInvalidRenderOutput(t *testing.T) {
	testCases := []struct {
		name        string
		args        []string
		wantErr     string
		wantRequest bool
		wantStdout  string
	}{
		{
			name:        "explicit json wins over the invalid default",
			args:        []string{"workspaces", "--output", "json"},
			wantRequest: true,
			wantStdout:  `"name": "acme-corp"`,
		},
		{
			name:        "explicit yaml wins over the invalid default",
			args:        []string{"workspaces", "-o", "yaml"},
			wantRequest: true,
			wantStdout:  "name: acme-corp",
		},
		{
			name:        "explicit text wins over the invalid default",
			args:        []string{"workspaces", "-o", "text"},
			wantRequest: true,
			wantStdout:  "acme-corp",
		},
		{
			name:    "invalid default is still refused when selected",
			args:    []string{"workspaces"},
			wantErr: "invalid RENDER_OUTPUT value: qa-invalid-output",
		},
		{
			name:    "invalid explicit flag is still refused",
			args:    []string{"workspaces", "-o", "xml"},
			wantErr: "invalid output format: xml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := renderapi.NewServer(t)
			server.Owners.Add(renderapi.NewOwner(client.Owner{Name: "acme-corp", Email: "admin@acme.com"}))
			t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
			t.Setenv("RENDER_HOST", server.URL())
			t.Setenv("RENDER_API_KEY", "test-api-key")
			t.Setenv("RENDER_WORKSPACE", "")
			t.Setenv("RENDER_OUTPUT", "qa-invalid-output")

			c, err := client.NewClientWithResponses(server.URL())
			require.NoError(t, err)
			deps := dependencies.New(c)

			root := newRootCmd()
			// A fresh copy keeps an earlier case's merged --output state from
			// leaking into this one.
			cmd := *workspacesCmd
			cmd.ResetFlags()
			root.AddCommand(&cmd)
			setupRootCmdPersistentRun(root, deps)

			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs(tc.args)

			execErr := root.Execute()

			if tc.wantErr != "" {
				require.EqualError(t, execErr, tc.wantErr)
				assert.Contains(t, stderr.String(), tc.wantErr)
				assert.Empty(t, stdout.String())
			} else {
				require.NoError(t, execErr)
				assert.Contains(t, stdout.String(), tc.wantStdout)
			}
			assert.Equal(t, tc.wantRequest, server.HasRequest("GET", "/owners"))
		})
	}
}

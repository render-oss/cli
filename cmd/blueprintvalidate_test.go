package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

func TestBlueprintValidateKeepsFileCompletion(t *testing.T) {
	requireCompletionDirective(t, []string{"blueprints", "validate", ""}, cobra.ShellCompDirectiveDefault)
}

const testBlueprintWorkspaceID = "wrk-blueprint-test"

// writeTestBlueprint writes blueprint content to a temp file and returns its path.
func writeTestBlueprint(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "render.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// blueprintValidateHarness starts an httptest server that answers
// POST /blueprints/validate with the given handler and points the CLI's
// env-based config at it so runBlueprintValidate's NewDefaultClient hits it.
// It also captures the workspace ID sent in the multipart form.
type blueprintValidateHarness struct {
	t               *testing.T
	server          *httptest.Server
	sentWorkspaceID string
}

func newBlueprintValidateHarness(t *testing.T, validateHandler http.HandlerFunc) *blueprintValidateHarness {
	t.Helper()

	h := &blueprintValidateHarness{t: t}
	h.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/blueprints/validate" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err == nil {
			h.sentWorkspaceID = r.FormValue("ownerId")
		}
		validateHandler(w, r)
	}))
	t.Cleanup(h.server.Close)

	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_API_KEY", "test-api-key")
	t.Setenv("RENDER_HOST", h.server.URL)
	t.Setenv("RENDER_WORKSPACE", testBlueprintWorkspaceID)

	return h
}

// execute invokes `render blueprints validate <blueprintPath>` with extraArgs
// appended and returns the captured output plus the execution error.
func (h *blueprintValidateHarness) execute(blueprintPath string, extraArgs ...string) (CommandResult, error) {
	h.t.Helper()

	c, err := client.NewClientWithResponses(h.server.URL)
	require.NoError(h.t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{StdinTTY: false, StdoutTTY: false, StderrTTY: false}, nil
	}

	root := newRootCmd()
	root.AddCommand(blueprintsCmd)
	setupRootCmdPersistentRun(root, deps)

	args := append([]string{"blueprints", "validate", blueprintPath}, extraArgs...)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)

	execErr := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, execErr
}

func jsonBlueprintResponse(valid bool, errors []map[string]any) map[string]any {
	resp := map[string]any{"valid": valid}
	if errors != nil {
		resp["errors"] = errors
	}
	return resp
}

func writeValidateResponse(t *testing.T, w http.ResponseWriter, body map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(body))
}

func TestBlueprintValidate_Valid_SucceedsInAllNonInteractiveModes(t *testing.T) {
	for _, format := range []string{"json", "yaml", "text"} {
		t.Run(format, func(t *testing.T) {
			harness := newBlueprintValidateHarness(t, func(w http.ResponseWriter, _ *http.Request) {
				writeValidateResponse(t, w, jsonBlueprintResponse(true, nil))
			})

			blueprintPath := writeTestBlueprint(t, "services:\n  - type: web\n    name: api\n")
			result, err := harness.execute(blueprintPath, "--output", format)
			require.NoError(t, err, "valid blueprint should not error in %s mode", format)
			assert.Equal(t, testBlueprintWorkspaceID, harness.sentWorkspaceID, "workspace ID should be sent to the validate endpoint")
			assert.NotEmpty(t, result.Stdout)
		})
	}
}

func TestBlueprintValidate_Invalid_ExitsNonZeroInAllNonInteractiveModes(t *testing.T) {
	validationErr := map[string]any{"error": "service 'api' is missing a required 'name' field"}

	for _, format := range []string{"json", "yaml", "text"} {
		t.Run(format, func(t *testing.T) {
			harness := newBlueprintValidateHarness(t, func(w http.ResponseWriter, _ *http.Request) {
				writeValidateResponse(t, w, jsonBlueprintResponse(false, []map[string]any{validationErr}))
			})

			blueprintPath := writeTestBlueprint(t, "services:\n  - type: web\n")
			result, err := harness.execute(blueprintPath, "--output", format)

			require.Error(t, err, "invalid blueprint should fail in %s mode so scripts see a non-zero exit", format)
			assert.Contains(t, err.Error(), "has validation errors")
			assert.Contains(t, result.Stdout, "missing a required 'name' field", "validation errors should still be printed in %s mode", format)
		})
	}
}

func TestBlueprintValidate_Invalid_NonInteractiveExitsNonZero(t *testing.T) {
	harness := newBlueprintValidateHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		writeValidateResponse(t, w, jsonBlueprintResponse(false, nil))
	})

	blueprintPath := writeTestBlueprint(t, "services:\n  - type: web\n  - type: web\n")

	// json: the structured result is printed, and the error drives a non-zero exit.
	result, err := harness.execute(blueprintPath, "--output", "json")
	require.Error(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body), "expected JSON output, got: %s", result.Stdout)
	assert.Equal(t, false, body["valid"])
	assert.Contains(t, err.Error(), "has validation errors")
}

func TestBlueprintValidate_EmptyResponse_ReturnsErrorWithoutPanicking(t *testing.T) {
	harness := newBlueprintValidateHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	blueprintPath := writeTestBlueprint(t, "services: []")
	_, err := harness.execute(blueprintPath, "--output", "json")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response")
}

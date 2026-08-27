package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

func TestBlueprintValidateKeepsFileCompletion(t *testing.T) {
	requireCompletionDirective(t, []string{"blueprints", "validate", ""}, cobra.ShellCompDirectiveDefault)
}

func TestBlueprintValidateNonInteractiveExitCode(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		result := bptypes.ValidateBlueprintResponse{Valid: true}

		output := requireBlueprintValidateResult(t, result, command.TEXT, 0)

		require.Empty(t, output.Stderr)
		require.JSONEq(t, `{"valid": true}`, output.Stdout)
	})

	t.Run("invalid", func(t *testing.T) {
		validationErrors := []bptypes.ValidationError{{Error: "services[0].type is required"}}
		result := bptypes.ValidateBlueprintResponse{
			Valid:  false,
			Errors: &validationErrors,
		}
		expectedJSON := `{
  "errors": [
    {
      "error": "services[0].type is required"
    }
  ],
  "valid": false
}`

		t.Run("text renders JSON", func(t *testing.T) {
			output := requireBlueprintValidateResult(t, result, command.TEXT, 1)

			require.Empty(t, output.Stderr)
			require.JSONEq(t, expectedJSON, output.Stdout)
		})

		t.Run("json", func(t *testing.T) {
			output := requireBlueprintValidateResult(t, result, command.JSON, 1)

			require.Empty(t, output.Stderr)
			require.JSONEq(t, expectedJSON, output.Stdout)
		})

		t.Run("yaml", func(t *testing.T) {
			output := requireBlueprintValidateResult(t, result, command.YAML, 1)

			require.Empty(t, output.Stderr)
			require.YAMLEq(t, expectedJSON, output.Stdout)
			require.False(t, json.Valid([]byte(output.Stdout)), "YAML output must not use JSON syntax")
		})
	})
}

func TestBlueprintValidateRejectsUnexpectedResponseBody(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Blueprints.RespondWithRawValidation("text/html", []byte("<html>not the API</html>"))

	execution, output := executeBlueprintValidate(t, server, command.TEXT)

	require.Equal(t, 1, execution.ExitCode)
	require.Empty(t, output.Stdout)
	require.Contains(t, output.Stderr, "unexpected response from server")
	require.Contains(t, output.Stderr, "<html>not the API</html>")
}

func requireBlueprintValidateResult(t *testing.T, result bptypes.ValidateBlueprintResponse, outputFormat command.Output, wantExitCode int) CommandResult {
	t.Helper()
	server := renderapi.NewServer(t)
	server.Blueprints.RespondWithValidation(result)
	execution, output := executeBlueprintValidate(t, server, outputFormat)

	require.Equal(t, wantExitCode, execution.ExitCode)
	require.Equal(t, outputFormat, *execution.OutputFormat)
	require.True(t, server.HasRequest("POST", "/blueprints/validate"))

	return output
}

func executeBlueprintValidate(t *testing.T, server *renderapi.Server, outputFormat command.Output) (command.ExecutionResult, CommandResult) {
	t.Helper()

	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_HOST", server.URL())
	t.Setenv("RENDER_API_KEY", "test-api-key")

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{StdinTTY: false, StdoutTTY: false, StderrTTY: false}, nil
	}

	root := newRootCmd()
	blueprints := &cobra.Command{Use: "blueprints", GroupID: GroupManagement.ID}
	blueprints.AddCommand(newBlueprintValidateCmd())
	root.AddCommand(blueprints)
	setupRootCmdPersistentRun(root, deps)

	blueprintPath := filepath.Join(t.TempDir(), "render.yaml")
	require.NoError(t, os.WriteFile(blueprintPath, []byte("services: []\n"), 0o600))

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"blueprints", "validate", blueprintPath, "--workspace", "tea-test", "--output", string(outputFormat)})

	execution := runExecution(root, time.Now())
	output := CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}
	return execution, output
}

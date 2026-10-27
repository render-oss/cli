package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

func executeFlowCommand(t *testing.T, server *renderapi.Server, args ...string) (CommandResult, error) {
	t.Helper()
	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_API_KEY", "test-api-key")

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{}, nil
	}

	root := newRootCmd()
	setupFlowCommands(root, deps)
	setupRootCmdPersistentRun(root, deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)

	execErr := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, execErr
}

// seedFlowRun adds a failed flow run shaped like the API's GET /flow-runs
// response, with the given errorDetails object (or none).
func seedFlowRun(server *renderapi.Server, errorDetails string) string {
	const id = "frn-test1234"
	body := `{
		"id": "` + id + `",
		"flowId": "flw-test1234",
		"status": "failed",
		"input": ["v2.1"],
		"results": [],
		"attempts": [],
		"startedAt": "2026-10-20T01:00:00Z",
		"completedAt": "2026-10-20T01:00:06Z",
		"error": "raw error"`
	if errorDetails != "" {
		body += `, "errorDetails": ` + errorDetails
	}
	body += `}`
	server.FlowRuns.Add(&renderapi.FlowRun{Id: id, Body: json.RawMessage(body)})
	return id
}

func TestFlowRunShow_DeterminismError(t *testing.T) {
	server := renderapi.NewServer(t)
	id := seedFlowRun(server, `{
		"code": "flow_determinism_error",
		"message": "replay diverged",
		"kind": "type",
		"seq": 3,
		"subKey": "notify",
		"flowAttempt": 0,
		"expected": "inline",
		"actual": "sleep"
	}`)

	result, err := executeFlowCommand(t, server, "flows", "runs", "show", id, "--output", "text")
	require.NoError(t, err)

	assert.Equal(t, "Flow run frn-test1234\n"+
		"Status: failed\n"+
		"Started at: 2026-10-20T01:00:00Z\n"+
		"Completed at: 2026-10-20T01:00:06Z\n"+
		`Input: ["v2.1"]`+"\n"+
		"Error: Determinism error at step 3 (notify): the step no longer matches the recorded step\n"+
		"  Recorded by attempt: 1\n"+
		"  Expected: inline\n"+
		"  Actual: sleep\n", result.Stdout)
}

func TestFlowRunShow_LimitError(t *testing.T) {
	server := renderapi.NewServer(t)
	id := seedFlowRun(server, `{
		"code": "flow_payload_too_large",
		"message": "payload too large",
		"limit": 256000,
		"value": 300000
	}`)

	result, err := executeFlowCommand(t, server, "flows", "runs", "show", id, "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "Error: Step payload too large: a step's input, output, or error is over the size limit in bytes\n"+
		"  Limit: 256,000 bytes\n"+
		"  Value: 300,000 bytes\n")
	assert.NotContains(t, result.Stdout, "raw error")
}

func TestFlowRunShow_JSONKeepsErrorDetails(t *testing.T) {
	server := renderapi.NewServer(t)
	id := seedFlowRun(server, `{"code": "flow_step_limit_exceeded", "message": "step limit exceeded", "limit": 10000, "value": 10001}`)

	result, err := executeFlowCommand(t, server, "flows", "runs", "show", id, "--output", "json")
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
	assert.Equal(t, id, body["id"])
	assert.Equal(t, []any{"v2.1"}, body["input"])
	assert.Equal(t, map[string]any{
		"code":    "flow_step_limit_exceeded",
		"message": "step limit exceeded",
		"limit":   float64(10000),
		"value":   float64(10001),
	}, body["errorDetails"])
}

func TestFlowRunShow_PlainErrorWithoutDetails(t *testing.T) {
	server := renderapi.NewServer(t)
	id := seedFlowRun(server, "")

	result, err := executeFlowCommand(t, server, "flows", "runs", "show", id, "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "Error: raw error\n")
	assert.NotContains(t, result.Stdout, "Results")
}

func TestFlowRunShow_NotFound(t *testing.T) {
	server := renderapi.NewServer(t)

	_, err := executeFlowCommand(t, server, "flows", "runs", "show", "frn-missing", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

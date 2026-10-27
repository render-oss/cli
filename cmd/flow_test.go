package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

var flowActiveWorkspaceID = testids.WorkspaceID("flow-active")

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

func withFlowWorkspace(t *testing.T) {
	t.Helper()
	t.Setenv("RENDER_WORKSPACE", flowActiveWorkspaceID)
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

func TestFlowList_FiltersAndWorkspace(t *testing.T) {
	withFlowWorkspace(t)
	server := renderapi.NewServer(t)
	server.Flows.Add(&renderapi.Flow{
		Id: "flw-1",
		Body: json.RawMessage(`{
			"id": "flw-1",
			"name": "my-flow",
			"workflowId": "wfl-1",
			"workflowVersionId": "wfv-1",
			"createdAt": "2026-10-20T01:00:00Z"
		}`),
	})

	result, err := executeFlowCommand(t, server,
		"flows", "list",
		"--flow-slug", "my-workflow/my-flow",
		"--workflow", "wfl-1",
		"--workflow-version", "wfv-1",
		"--output", "text",
	)
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "flw-1")
	assert.Contains(t, result.Stdout, "my-flow")

	require.Len(t, server.Flows.ListCalls, 1)
	call := server.Flows.ListCalls[0]
	assert.Equal(t, []string{flowActiveWorkspaceID}, call.OwnerIds)
	assert.Equal(t, []string{"my-workflow/my-flow"}, call.FlowSlugs)
	assert.Equal(t, []string{"wfl-1"}, call.WorkflowIds)
	assert.Equal(t, []string{"wfv-1"}, call.WorkflowVersionId)
	assert.Equal(t, "100", call.Limit)
	assert.Equal(t, "", call.Cursor)
}

func TestFlowList_RequiresWorkspace(t *testing.T) {
	t.Setenv("RENDER_WORKSPACE", "")
	server := renderapi.NewServer(t)

	_, err := executeFlowCommand(t, server, "flows", "list", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no workspace set")
	assert.Empty(t, server.Flows.ListCalls)
}

func TestFlowList_PaginationFlags(t *testing.T) {
	withFlowWorkspace(t)
	server := renderapi.NewServer(t)
	// Seed exactly two flows and ask for a page of two so the fake returns a
	// cursor the next-page hint can echo.
	server.Flows.Add(&renderapi.Flow{Id: "flw-a", Body: json.RawMessage(`{"id":"flw-a"}`)})
	server.Flows.Add(&renderapi.Flow{Id: "flw-b", Body: json.RawMessage(`{"id":"flw-b"}`)})

	result, err := executeFlowCommand(t, server,
		"flows", "list",
		"--limit", "2",
		"--cursor", "c-prev",
		"--output", "text",
	)
	require.NoError(t, err)
	call := server.Flows.ListCalls[0]
	assert.Equal(t, "2", call.Limit)
	assert.Equal(t, "c-prev", call.Cursor)
	assert.Contains(t, result.Stderr, "More results available")
	assert.Contains(t, result.Stderr, "--cursor c1")
}

func TestFlowList_RejectsZeroLimit(t *testing.T) {
	withFlowWorkspace(t)
	server := renderapi.NewServer(t)

	_, err := executeFlowCommand(t, server, "flows", "list", "--limit", "0", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--limit must be greater than 0")
}

func TestFlowList_Empty(t *testing.T) {
	withFlowWorkspace(t)
	server := renderapi.NewServer(t)

	result, err := executeFlowCommand(t, server, "flows", "list", "--output", "json")
	require.NoError(t, err)

	var decoded []any
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &decoded))
	assert.Empty(t, decoded)
}

func TestFlowGet(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Flows.Add(&renderapi.Flow{
		Id: "flw-abc",
		Body: json.RawMessage(`{
			"id": "flw-abc",
			"name": "my-flow",
			"createdAt": "2026-10-20T01:00:00Z"
		}`),
	})

	result, err := executeFlowCommand(t, server, "flows", "get", "flw-abc", "--output", "text")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "Flow flw-abc")
	assert.Contains(t, result.Stdout, "Name: my-flow")
}

func TestFlowGet_NotFound(t *testing.T) {
	server := renderapi.NewServer(t)
	_, err := executeFlowCommand(t, server, "flows", "get", "flw-missing", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestFlowRunList_FiltersAndWorkspace(t *testing.T) {
	withFlowWorkspace(t)
	server := renderapi.NewServer(t)
	server.FlowRuns.Add(&renderapi.FlowRun{
		Id: "frn-1",
		Body: json.RawMessage(`{
			"id": "frn-1",
			"flowId": "flw-1",
			"status": "running"
		}`),
	})

	result, err := executeFlowCommand(t, server,
		"flows", "runs", "list",
		"--flow-slug", "my-workflow/my-flow",
		"--flow", "flw-1",
		"--status", "running",
		"--status", "pending",
		"--workflow", "wfl-1",
		"--workflow-version", "wfv-1",
		"--output", "text",
	)
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "frn-1")

	require.Len(t, server.FlowRuns.ListCalls, 1)
	call := server.FlowRuns.ListCalls[0]
	assert.Equal(t, []string{flowActiveWorkspaceID}, call.OwnerIds)
	assert.Equal(t, []string{"my-workflow/my-flow"}, call.FlowSlugs)
	assert.Equal(t, []string{"flw-1"}, call.FlowIds)
	assert.Equal(t, []string{"running", "pending"}, call.Statuses)
	assert.Equal(t, []string{"wfl-1"}, call.WorkflowIds)
	assert.Equal(t, []string{"wfv-1"}, call.WorkflowVersionId)
	assert.Equal(t, "100", call.Limit)
}

func TestFlowRunList_RequiresWorkspace(t *testing.T) {
	t.Setenv("RENDER_WORKSPACE", "")
	server := renderapi.NewServer(t)

	_, err := executeFlowCommand(t, server, "flows", "runs", "list", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no workspace set")
	assert.Empty(t, server.FlowRuns.ListCalls)
}

func TestFlowRunList_PaginationFlags(t *testing.T) {
	withFlowWorkspace(t)
	server := renderapi.NewServer(t)
	server.FlowRuns.Add(&renderapi.FlowRun{Id: "frn-a", Body: json.RawMessage(`{"id":"frn-a","flowId":"flw-1","status":"running"}`)})

	result, err := executeFlowCommand(t, server,
		"flows", "runs", "list",
		"--limit", "1",
		"--cursor", "c-prev",
		"--output", "text",
	)
	require.NoError(t, err)
	call := server.FlowRuns.ListCalls[0]
	assert.Equal(t, "1", call.Limit)
	assert.Equal(t, "c-prev", call.Cursor)
	assert.Contains(t, result.Stderr, "More results available")
	assert.Contains(t, result.Stderr, "--cursor c0")
}

func TestFlowRunStart_FromSlug(t *testing.T) {
	server := renderapi.NewServer(t)
	server.FlowRuns.RespondWithBodyForStart(json.RawMessage(`{
		"id": "frn-new",
		"flowId": "flw-123",
		"status": "pending"
	}`))

	result, err := executeFlowCommand(t, server,
		"flows", "runs", "start", "my-workflow/my-flow",
		"--input", `["arg1","arg2"]`,
		"--idempotency-key", "idem-1",
		"--output", "text",
	)
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "Started flow run frn-new for my-workflow/my-flow")

	require.Len(t, server.FlowRuns.StartedRuns, 1)
	var body map[string]any
	require.NoError(t, json.Unmarshal(server.FlowRuns.StartedRuns[0], &body))
	assert.Equal(t, "my-workflow/my-flow", body["flow"])
	assert.Equal(t, []any{"arg1", "arg2"}, body["input"])
	assert.Equal(t, "idem-1", body["idempotencyKey"])
}

func TestFlowRunStart_FromFlag(t *testing.T) {
	server := renderapi.NewServer(t)

	_, err := executeFlowCommand(t, server,
		"flows", "runs", "start",
		"--flow", "my-workflow/my-flow",
		"--input", `{"key":"value"}`,
		"--output", "json",
	)
	require.NoError(t, err)

	require.Len(t, server.FlowRuns.StartedRuns, 1)
	var body map[string]any
	require.NoError(t, json.Unmarshal(server.FlowRuns.StartedRuns[0], &body))
	assert.Equal(t, "my-workflow/my-flow", body["flow"])
	assert.Equal(t, map[string]any{"key": "value"}, body["input"])
}

func TestFlowRunStart_MissingFlow(t *testing.T) {
	server := renderapi.NewServer(t)
	_, err := executeFlowCommand(t, server, "flows", "runs", "start", "--input", `[]`, "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flow slug is required")
}

func TestFlowRunStart_MissingInput(t *testing.T) {
	server := renderapi.NewServer(t)
	_, err := executeFlowCommand(t, server, "flows", "runs", "start", "my-workflow/my-flow", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--input or --input-file is required")
}

func TestFlowRunStart_InvalidJSON(t *testing.T) {
	server := renderapi.NewServer(t)
	_, err := executeFlowCommand(t, server,
		"flows", "runs", "start", "my-workflow/my-flow",
		"--input", "not json",
		"--output", "text",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse --input as JSON")
}

func TestFlowRunCancel(t *testing.T) {
	server := renderapi.NewServer(t)
	server.FlowRuns.Add(&renderapi.FlowRun{
		Id:   "frn-cancel",
		Body: json.RawMessage(`{"id":"frn-cancel","flowId":"flw-1","status":"running"}`),
	})

	result, err := executeFlowCommand(t, server, "flows", "runs", "cancel", "frn-cancel", "--output", "text")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "Cancelled flow run frn-cancel")
	assert.True(t, server.HasRequest("DELETE", "/flow-runs/frn-cancel"))
}

func TestFlowRunCancel_JSONOutput(t *testing.T) {
	server := renderapi.NewServer(t)
	server.FlowRuns.Add(&renderapi.FlowRun{
		Id:   "frn-cancel",
		Body: json.RawMessage(`{"id":"frn-cancel","flowId":"flw-1","status":"running"}`),
	})

	result, err := executeFlowCommand(t, server, "flows", "runs", "cancel", "frn-cancel", "--output", "json")
	require.NoError(t, err)
	// Mirrors `deploys cancel` and `jobs cancel`: the API has no body to
	// return, so the message is encoded as a JSON string.
	var message string
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &message))
	assert.Equal(t, "Cancelled flow run frn-cancel", message)
}

func TestFlowRunCancel_NotFound(t *testing.T) {
	server := renderapi.NewServer(t)
	_, err := executeFlowCommand(t, server, "flows", "runs", "cancel", "frn-missing", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestFlowRunStepsList(t *testing.T) {
	server := renderapi.NewServer(t)
	server.FlowRuns.AddStep(&renderapi.FlowRunStep{
		FlowRunID: "frn-1",
		Seq:       0,
		Body: json.RawMessage(`{
			"seq": 0,
			"stepKey": "step-0",
			"name": "fetch",
			"type": "inline",
			"status": "succeeded",
			"startedAt": "2026-10-20T01:00:00Z",
			"completedAt": "2026-10-20T01:00:01Z",
			"flowRunAttempt": 0
		}`),
	})
	server.FlowRuns.AddStep(&renderapi.FlowRunStep{
		FlowRunID: "frn-1",
		Seq:       1,
		Body: json.RawMessage(`{
			"seq": 1,
			"stepKey": "step-1",
			"type": "sleep",
			"status": "running",
			"startedAt": "2026-10-20T01:00:01Z",
			"flowRunAttempt": 0
		}`),
	})

	result, err := executeFlowCommand(t, server, "flows", "runs", "steps", "list", "frn-1", "--output", "text")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "fetch")
	assert.Contains(t, result.Stdout, "inline")
	assert.Contains(t, result.Stdout, "sleep")

	require.Len(t, server.FlowRuns.StepListCalls, 1)
	assert.Equal(t, "100", server.FlowRuns.StepListCalls[0].Limit)
}

func TestFlowRunStepsList_PaginationFlags(t *testing.T) {
	server := renderapi.NewServer(t)
	server.FlowRuns.AddStep(&renderapi.FlowRunStep{
		FlowRunID: "frn-1",
		Seq:       0,
		Body:      json.RawMessage(`{"seq":0,"stepKey":"k","type":"inline","status":"succeeded","flowRunAttempt":0}`),
	})

	result, err := executeFlowCommand(t, server,
		"flows", "runs", "steps", "list", "frn-1",
		"--limit", "1",
		"--cursor", "c-prev",
		"--output", "text",
	)
	require.NoError(t, err)
	require.Len(t, server.FlowRuns.StepListCalls, 1)
	call := server.FlowRuns.StepListCalls[0]
	assert.Equal(t, "1", call.Limit)
	assert.Equal(t, "c-prev", call.Cursor)
	assert.Contains(t, result.Stderr, "More results available")
	assert.Contains(t, result.Stderr, "--cursor c0")
}

func TestFlowRunStepShow(t *testing.T) {
	server := renderapi.NewServer(t)
	server.FlowRuns.AddStep(&renderapi.FlowRunStep{
		FlowRunID: "frn-1",
		Seq:       3,
		Body: json.RawMessage(`{
			"seq": 3,
			"stepKey": "step-3",
			"name": "notify",
			"type": "detached",
			"status": "failed",
			"startedAt": "2026-10-20T01:00:00Z",
			"completedAt": "2026-10-20T01:00:05Z",
			"flowRunAttempt": 1,
			"childTaskRunId": "trn-child",
			"error": "boom",
			"attempts": []
		}`),
	})

	result, err := executeFlowCommand(t, server, "flows", "runs", "steps", "show", "frn-1", "3", "--output", "text")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "Step 3 (step-3)")
	assert.Contains(t, result.Stdout, "Name: notify")
	assert.Contains(t, result.Stdout, "Type: detached")
	assert.Contains(t, result.Stdout, "Status: failed")
	assert.Contains(t, result.Stdout, "Child task run: trn-child")
	assert.Contains(t, result.Stdout, "Error: boom")
}

func TestFlowRunStepShow_BadSeq(t *testing.T) {
	server := renderapi.NewServer(t)
	_, err := executeFlowCommand(t, server, "flows", "runs", "steps", "show", "frn-1", "abc", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid step sequence")
}

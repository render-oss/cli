package cmd

import (
	"encoding/json"
	"net/url"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/flowrun"
	"github.com/stretchr/testify/require"
)

const attemptFailure = `{"attempt":0,"flowAttempt":2,"startedAt":"2026-11-01T00:00:00Z","completedAt":"2026-11-01T00:00:01Z","retryAt":"2026-11-01T00:00:02Z"}`

func seedAttemptStep(server *renderapi.Server, status, kind, failures, key string) {
	server.FlowRuns.AddStep(&renderapi.FlowRunStep{FlowRunID: "frn-test", Seq: 3, Key: key, Body: json.RawMessage(`{"seq":3,"type":"` + kind + `","status":"` + status + `","startedAt":"2026-11-01T00:00:00Z","completedAt":"2026-11-01T00:00:03Z","input":["charge"],"result":42` + failures + `}`)})
	server.FlowRuns.StepAttempts = append(server.FlowRuns.StepAttempts, &renderapi.FlowRunStepAttempt{FlowRunID: "frn-test", Seq: 3, Key: key, Attempt: 0, Body: json.RawMessage(`{"attempt":0,"flowAttempt":2,"startedAt":"2026-11-01T00:00:00Z","completedAt":"2026-11-01T00:00:01Z","error":"charge rejected"}`)})
}

func TestFlowStepAttemptsCompleteness(t *testing.T) {
	for _, tc := range []struct {
		name, status, kind, failures, wantErr string
		count                                 int
		last                                  int64
		recorded                              bool
	}{
		{"success", "succeeded", "inline", `,"failedAttempts":[` + attemptFailure + `,{"attempt":1,"flowAttempt":2,"startedAt":"2026-11-01T00:00:01Z","completedAt":"2026-11-01T00:00:02Z"}]`, "", 3, 3, false},
		{"retry", "running", "inline", `,"failedAttempts":[` + attemptFailure + `]`, "", 2, 2, false},
		{"permanent", "failed", "inline", `,"failedAttempts":[` + attemptFailure + `]`, "", 1, 1, true},
		{"no failures", "succeeded", "inline", `,"failedAttempts":[]`, "", 1, 1, false},
		{"initial incomplete", "running", "inline", `,"failedAttempts":[]`, "", 1, 1, false},
		{"gaps", "succeeded", "inline", `,"failedAttempts":[{"attempt":4,"flowAttempt":3,"startedAt":"2026-11-01T00:00:01Z","completedAt":"2026-11-01T00:00:02Z"},` + attemptFailure + `]`, "", 3, 6, false},
		{"old server", "succeeded", "inline", "", "omitted failedAttempts", 0, 0, false},
		{"detached", "succeeded", "detached", `,"failedAttempts":[]`, "only supported for inline", 0, 0, false},
		{"sleep", "succeeded", "sleep", `,"failedAttempts":[]`, "only supported for inline", 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := renderapi.NewServer(t)
			seedAttemptStep(server, tc.status, tc.kind, tc.failures, "")
			result, err := executeFlowCommand(t, server, "flows", "runs", "steps", "attempts", "list", "frn-test", "3", "-o", "json")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			var body []map[string]any
			require.NoError(t, json.Unmarshal([]byte(result.Stdout), &body))
			require.Len(t, body, tc.count)
			last := body[len(body)-1]
			require.EqualValues(t, tc.last, last["attempt"])
			require.Equal(t, tc.recorded, last["recorded"])
			if !tc.recorded {
				require.NotContains(t, last, "startedAt")
				require.NotContains(t, last, "flowAttempt")
			}
		})
	}
}

func TestFlowStepAttemptShowIdentityAndFormats(t *testing.T) {
	for _, format := range []string{"text", "json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			server := renderapi.NewServer(t)
			key := "charge / ?&雪"
			seedAttemptStep(server, "succeeded", "inline", `,"failedAttempts":[`+attemptFailure+`]`, key)
			result, err := executeFlowCommand(t, server, "flows", "runs", "steps", "attempts", "show", "frn-test", "3", "1", "--step-key", key, "-o", format)
			require.NoError(t, err)
			require.Contains(t, result.Stdout, "charge rejected")
			if format == "text" {
				require.Contains(t, result.Stdout, "Attempt: 1")
				require.Contains(t, result.Stdout, "Flow attempt: 3")
				require.Contains(t, result.Stdout, "Recorded: true")
			} else if format == "json" {
				require.Contains(t, result.Stdout, `"attempt": 1`)
				require.Contains(t, result.Stdout, `"flowAttempt": 3`)
			} else {
				require.Contains(t, result.Stdout, "attempt: 1")
				require.Contains(t, result.Stdout, "flowAttempt: 3")
				require.Contains(t, result.Stdout, "recorded: true")
			}
			requests := server.Requests
			require.Len(t, requests, 2)
			u, err := url.Parse(requests[0].URI)
			require.NoError(t, err)
			require.Equal(t, "true", u.Query().Get("includeAttempts"))
			require.Equal(t, key, u.Query().Get("stepKey"))
			u, err = url.Parse(requests[1].URI)
			require.NoError(t, err)
			require.Equal(t, "/flow-runs/frn-test/steps/3/attempts/0", u.Path)
			require.Equal(t, key, u.Query().Get("stepKey"))
			result, err = executeFlowCommand(t, server, "flows", "runs", "steps", "attempts", "show", "frn-test", "3", "2", "--step-key", key, "-o", "json")
			require.NoError(t, err)
			require.Contains(t, result.Stdout, `"recorded": false`)
			require.Contains(t, result.Stdout, `"result": 42`)
			require.NotContains(t, result.Stdout, "startedAt")
			require.NotContains(t, result.Stdout, "flowAttempt")
			_, err = executeFlowCommand(t, server, "flows", "runs", "steps", "attempts", "show", "frn-test", "3", "3", "--step-key", key, "-o", "json")
			require.ErrorContains(t, err, "attempt 3 not found")
		})
	}
}

func TestFlowStepAttemptInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"3", ""}, {"3", "0"}, {"3", "-1"}, {"3", "4294967297"}, {"3", "18446744073709551616"}, {"-1", "1"}, {"4294967296", "1"}} {
		t.Run(args[0]+"_"+args[1], func(t *testing.T) {
			server := renderapi.NewServer(t)
			_, err := executeFlowCommand(t, server, "flows", "runs", "steps", "attempts", "show", "frn-test", args[0], args[1], "-o", "json")
			require.Error(t, err)
			require.Empty(t, server.Requests)
		})
	}
	lastAttempt := "4294967296"
	input := FlowRunStepAttemptInput{StepSeq: "2147483647", Attempt: &lastAttempt}
	seq, attempt, err := input.Parse()
	require.NoError(t, err)
	require.EqualValues(t, 2147483647, seq)
	require.EqualValues(t, int64(4294967295), attempt)
}

func TestFlowStepAttemptMissingStep(t *testing.T) {
	server := renderapi.NewServer(t)
	_, err := executeFlowCommand(t, server, "flows", "runs", "steps", "attempts", "list", "frn-missing", "3", "-o", "json")
	require.Error(t, err)
}

func TestFlowStepAttemptGapNotAddressable(t *testing.T) {
	server := renderapi.NewServer(t)
	seedAttemptStep(server, "succeeded", "inline", `,"failedAttempts":[{"attempt":4,"flowAttempt":2,"startedAt":"2026-11-01T00:00:00Z","completedAt":"2026-11-01T00:00:01Z"}]`, "")
	_, err := executeFlowCommand(t, server, "flows", "runs", "steps", "attempts", "show", "frn-test", "3", "1", "-o", "json")
	require.ErrorContains(t, err, "attempt 1 not found")
}

func TestFlowStepAttemptsTextList(t *testing.T) {
	server := renderapi.NewServer(t)
	seedAttemptStep(server, "succeeded", "inline", `,"failedAttempts":[`+attemptFailure+`]`, "")
	result, err := executeFlowCommand(t, server, "flows", "runs", "steps", "attempts", "list", "frn-test", "3", "-o", "text")
	require.NoError(t, err)
	require.Contains(t, result.Stdout, "Attempt: 1")
	require.Contains(t, result.Stdout, "Attempt: 2")
	require.Contains(t, result.Stdout, "Recorded: false")
	require.Contains(t, flowrun.StepAttemptDetails(&flowrun.StepAttempt{Attempt: 4294967295}), "Attempt: 4294967296")
}

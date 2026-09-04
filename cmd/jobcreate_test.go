package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	clientjob "github.com/render-oss/cli/pkg/client/jobs"
	logclient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/job"
	"github.com/render-oss/cli/pkg/pointers"
)

type jobCommandTicker struct{ ticks chan time.Time }

func newJobCommandTicker(count int) *jobCommandTicker {
	ticker := &jobCommandTicker{ticks: make(chan time.Time, count)}
	for range count {
		ticker.ticks <- time.Time{}
	}
	return ticker
}

func (t *jobCommandTicker) C() <-chan time.Time { return t.ticks }
func (t *jobCommandTicker) Stop()               {}

func executeJobCreateCommand(t *testing.T, server *renderapi.Server, ctx context.Context, ticks int, args ...string) (CommandResult, error) {
	t.Helper()
	return executeJobCreateCommandWithTicker(t, server, ctx, newJobCommandTicker(ticks), args...)
}

func executeJobCreateCommandWithTicker(t *testing.T, server *renderapi.Server, ctx context.Context, ticker *jobCommandTicker, args ...string) (CommandResult, error) {
	t.Helper()
	root, stdout, stderr := prepareJobCreateCommand(t, server, ctx, ticker, args...)
	err := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

func prepareJobCreateCommand(t *testing.T, server *renderapi.Server, ctx context.Context, ticker *jobCommandTicker, args ...string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_API_KEY", "test-api-key")
	t.Setenv("RENDER_HOST", server.URL())
	t.Setenv("RENDER_WORKSPACE", testids.WorkspaceID("jobs"))

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{StdinTTY: false, StdoutTTY: false, StderrTTY: false}, nil
	}

	root := newRootCmd()
	jobs := &cobra.Command{Use: "jobs"}
	jobs.AddCommand(newJobCreateCmd(jobCreateCommandConfig{
		pollInterval: time.Hour,
		newTicker:    func(time.Duration) job.TickSource { return ticker },
	}))
	root.AddCommand(jobs)
	setupRootCmdPersistentRun(root, deps)
	root.SetContext(ctx)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(append([]string{"jobs", "create"}, args...))
	return root, stdout, stderr
}

func TestJobCreateTailImpliesWaitAndKeepsStructuredStdoutClean(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Jobs.QueueStatuses(
		pointers.From(clientjob.Running),
		pointers.From(clientjob.Running),
		pointers.From(clientjob.Succeeded),
	)
	now := time.Unix(100, 0).UTC()
	server.Logs.QueueStream(
		renderapi.LogStreamAttempt{
			Logs: []logclient.Log{
				{Id: "log-1", Message: "first line", Timestamp: now},
				{Id: "log-2", Message: "second line", Timestamp: now.Add(time.Second)},
			},
			CloseCode: websocket.CloseInternalServerErr,
		},
		renderapi.LogStreamAttempt{
			Logs: []logclient.Log{
				{Id: "log-2", Message: "second line", Timestamp: now.Add(time.Second)},
				{Id: "log-3", Message: "third line", Timestamp: now.Add(2 * time.Second)},
			},
			HoldOpen: true,
		},
	)
	ticker := newJobCommandTicker(0)
	root, stdout, stderr := prepareJobCreateCommand(t, server, context.Background(), ticker,
		"srv-jobcreate000000000", "--tail", "--output", "json")
	outcome := make(chan error, 1)
	go func() {
		outcome <- root.Execute()
	}()

	<-server.Logs.Opened()
	<-server.Logs.Closed()
	ticker.ticks <- time.Time{}
	<-server.Logs.Opened()
	ticker.ticks <- time.Time{}
	require.NoError(t, <-outcome)
	<-server.Logs.Closed()

	var final clientjob.Job
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &final))
	require.NotNil(t, final.Status)
	assert.Equal(t, clientjob.Succeeded, *final.Status)
	assert.NotContains(t, stdout.String(), "first line")
	assert.Contains(t, stderr.String(), "first line")
	assert.Contains(t, stderr.String(), "third line")
	assert.Equal(t, 1, strings.Count(stderr.String(), "second line"))
	assert.Equal(t, 1, server.Jobs.CreateRequestCount())
	assert.Equal(t, 3, server.Jobs.RetrieveRequestCount())
	assert.Equal(t, 2, server.Logs.StreamRequestCount())
	assert.NotEmpty(t, server.Logs.Query(1).Get("startTime"))
}

func TestJobCreateTailKeepsYAMLStdoutClean(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Jobs.QueueStatuses(
		pointers.From(clientjob.Running),
		pointers.From(clientjob.Running),
		pointers.From(clientjob.Succeeded),
	)
	server.Logs.QueueStream(
		renderapi.LogStreamAttempt{
			Logs: []logclient.Log{{
				Id: "log-yaml", Message: "yaml log line", Timestamp: time.Unix(100, 0).UTC(),
			}},
			CloseCode: websocket.CloseInternalServerErr,
		},
		renderapi.LogStreamAttempt{HoldOpen: true},
	)
	ticker := newJobCommandTicker(0)
	root, stdout, stderr := prepareJobCreateCommand(t, server, context.Background(), ticker,
		"srv-jobcreate000000000", "--tail", "--output", "yaml")
	result := make(chan error, 1)
	go func() { result <- root.Execute() }()
	<-server.Logs.Opened()
	<-server.Logs.Closed()
	ticker.ticks <- time.Time{}
	<-server.Logs.Opened()
	ticker.ticks <- time.Time{}
	require.NoError(t, <-result)
	<-server.Logs.Closed()

	var final map[string]any
	require.NoError(t, yaml.Unmarshal(stdout.Bytes(), &final))
	assert.Equal(t, "succeeded", final["status"])
	assert.NotContains(t, stdout.String(), "yaml log line")
	assert.Contains(t, stderr.String(), "yaml log line")
}

func TestJobCreateTailTextSeparatesLogsFromControlMessages(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Jobs.QueueStatuses(
		pointers.From(clientjob.Running),
		pointers.From(clientjob.Running),
		pointers.From(clientjob.Succeeded),
	)
	server.Logs.QueueStream(
		renderapi.LogStreamAttempt{
			Logs: []logclient.Log{{
				Id: "log-text", Message: "pipeline output", Timestamp: time.Unix(100, 0).UTC(),
			}},
			CloseCode: websocket.CloseInternalServerErr,
		},
		renderapi.LogStreamAttempt{HoldOpen: true},
	)
	ticker := newJobCommandTicker(0)
	root, stdout, stderr := prepareJobCreateCommand(t, server, context.Background(), ticker,
		"srv-jobcreate000000000", "--tail", "--output", "text")
	result := make(chan error, 1)
	go func() { result <- root.Execute() }()
	<-server.Logs.Opened()
	<-server.Logs.Closed()
	ticker.ticks <- time.Time{}
	<-server.Logs.Opened()
	ticker.ticks <- time.Time{}
	require.NoError(t, <-result)
	<-server.Logs.Closed()

	assert.Contains(t, stdout.String(), "pipeline output")
	assert.Contains(t, stdout.String(), "finished with status succeeded")
	assert.NotContains(t, stdout.String(), "Waiting for job")
	assert.Contains(t, stderr.String(), "Waiting for job")
	assert.NotContains(t, stderr.String(), "pipeline output")
}

func TestJobCreateNoWaitPreservesImmediateBehavior(t *testing.T) {
	server := renderapi.NewServer(t)
	result, err := executeJobCreateCommand(t, server, context.Background(), 0,
		"srv-jobcreate000000000", "--start-command", "echo immediate", "--output", "text")
	require.NoError(t, err)
	assert.Equal(t, "Created job job-fake0000000000000000 for srv-jobcreate000000000\n", result.Stdout)
	assert.Empty(t, result.Stderr)
	assert.Equal(t, 1, server.Jobs.CreateRequestCount())
	assert.Zero(t, server.Jobs.RetrieveRequestCount())
}

func TestJobCreateWaitReturnsFinalSucceededJob(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Jobs.QueueStatuses(
		pointers.From(clientjob.Pending),
		pointers.From(clientjob.Running),
		pointers.From(clientjob.Succeeded),
	)
	result, err := executeJobCreateCommand(t, server, context.Background(), 2,
		"srv-jobcreate000000000", "--wait", "--output", "json")
	require.NoError(t, err)
	assert.Zero(t, exitCodeFromError(err))
	var final clientjob.Job
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &final))
	require.NotNil(t, final.Status)
	assert.Equal(t, clientjob.Succeeded, *final.Status)
	assert.Contains(t, result.Stderr, "Waiting for job")
	assert.Equal(t, 1, server.Jobs.CreateRequestCount())
	assert.Equal(t, 3, server.Jobs.RetrieveRequestCount())
}

func TestJobCreateFailedAndCanceledReturnNonzero(t *testing.T) {
	for _, status := range []clientjob.JobStatus{clientjob.Failed, clientjob.Canceled} {
		t.Run(string(status), func(t *testing.T) {
			server := renderapi.NewServer(t)
			server.Jobs.QueueStatuses(pointers.From(status))
			result, err := executeJobCreateCommand(t, server, context.Background(), 0,
				"srv-jobcreate000000000", "--wait", "--output", "json")
			require.Error(t, err)
			var exitCoder command.ExitCoder
			require.ErrorAs(t, err, &exitCoder)
			assert.Equal(t, 1, exitCoder.ExitCode())
			assert.Equal(t, 1, exitCodeFromError(err))
			var final clientjob.Job
			require.NoError(t, json.Unmarshal([]byte(result.Stdout), &final))
			require.NotNil(t, final.Status)
			assert.Equal(t, status, *final.Status)
			assert.Equal(t, 1, server.Jobs.CreateRequestCount())
		})
	}
}

func TestJobCreateRetriesRetrieveButNeverCreate(t *testing.T) {
	t.Run("retrieve 503 is retried", func(t *testing.T) {
		server := renderapi.NewServer(t)
		server.Jobs.RespondRetrieveWith(503)
		server.Jobs.QueueStatuses(pointers.From(clientjob.Succeeded))
		_, err := executeJobCreateCommand(t, server, context.Background(), 1,
			"srv-jobcreate000000000", "--wait", "--output", "text")
		require.NoError(t, err)
		assert.Equal(t, 1, server.Jobs.CreateRequestCount())
		assert.Equal(t, 2, server.Jobs.RetrieveRequestCount())
	})

	t.Run("create 503 is not retried", func(t *testing.T) {
		server := renderapi.NewServer(t)
		server.Jobs.RespondCreateWith(503)
		_, err := executeJobCreateCommand(t, server, context.Background(), 3,
			"srv-jobcreate000000000", "--wait", "--output", "text")
		require.Error(t, err)
		assert.Equal(t, 1, server.Jobs.CreateRequestCount())
		assert.Zero(t, server.Jobs.RetrieveRequestCount())
	})
}

func TestJobCreateTimeoutAndValidation(t *testing.T) {
	t.Run("timeout stops polling", func(t *testing.T) {
		server := renderapi.NewServer(t)
		server.Jobs.QueueStatuses(pointers.From(clientjob.Running))
		_, err := executeJobCreateCommand(t, server, context.Background(), 0,
			"srv-jobcreate000000000", "--wait", "--timeout", "1ns", "--output", "text")
		require.ErrorContains(t, err, "timed out waiting for job")
		assert.Equal(t, 1, server.Jobs.CreateRequestCount())
	})

	t.Run("timeout requires wait or tail", func(t *testing.T) {
		server := renderapi.NewServer(t)
		_, err := executeJobCreateCommand(t, server, context.Background(), 0,
			"srv-jobcreate000000000", "--timeout", "1m", "--output", "text")
		require.EqualError(t, err, "--timeout requires --wait or --tail")
		assert.Zero(t, server.Jobs.CreateRequestCount())
	})

	t.Run("timeout must be positive", func(t *testing.T) {
		server := renderapi.NewServer(t)
		_, err := executeJobCreateCommand(t, server, context.Background(), 0,
			"srv-jobcreate000000000", "--wait", "--timeout", "0s", "--output", "text")
		require.EqualError(t, err, "--timeout must be positive")
		assert.Zero(t, server.Jobs.CreateRequestCount())
	})
}

func TestJobCreateContextCancellationStopsPolling(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Jobs.QueueStatuses(pointers.From(clientjob.Running))
	ctx, cancel := context.WithCancel(context.Background())
	ticker := newJobCommandTicker(0)
	root, _, _ := prepareJobCreateCommand(t, server, ctx, ticker,
		"srv-jobcreate000000000", "--wait", "--output", "text")
	result := make(chan error, 1)
	go func() { result <- root.Execute() }()
	<-server.Jobs.Retrieved()
	cancel()
	err := <-result
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), err)
	assert.Equal(t, 1, server.Jobs.CreateRequestCount())
	assert.Equal(t, 1, server.Jobs.RetrieveRequestCount())
}

func TestJobCreateTailCancellationStopsPollingAndStreaming(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Jobs.QueueStatuses(pointers.From(clientjob.Running))
	server.Logs.QueueStream(renderapi.LogStreamAttempt{HoldOpen: true})
	ctx, cancel := context.WithCancel(context.Background())
	ticker := newJobCommandTicker(0)
	root, _, _ := prepareJobCreateCommand(t, server, ctx, ticker,
		"srv-jobcreate000000000", "--tail", "--output", "text")
	result := make(chan error, 1)
	go func() { result <- root.Execute() }()
	<-server.Logs.Opened()
	<-server.Jobs.Retrieved()
	cancel()
	err := <-result
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), err)
	<-server.Logs.Closed()
	assert.Equal(t, 1, server.Jobs.CreateRequestCount())
	assert.Equal(t, 1, server.Jobs.RetrieveRequestCount())
}

func TestJobCreateRejectsMissingAndUnknownStatuses(t *testing.T) {
	unknown := clientjob.JobStatus("paused")
	for name, status := range map[string]*clientjob.JobStatus{"missing": nil, "unknown": &unknown} {
		t.Run(name, func(t *testing.T) {
			server := renderapi.NewServer(t)
			server.Jobs.QueueStatuses(status)
			_, err := executeJobCreateCommand(t, server, context.Background(), 0,
				"srv-jobcreate000000000", "--wait", "--output", "text")
			require.Error(t, err)
			var statusErr *job.StatusError
			require.ErrorAs(t, err, &statusErr)
			assert.Equal(t, 1, server.Jobs.RetrieveRequestCount())
		})
	}
}

func TestJobCreateWaitYAMLOutputIsOneDocument(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Jobs.QueueStatuses(pointers.From(clientjob.Succeeded))
	result, err := executeJobCreateCommand(t, server, context.Background(), 0,
		"srv-jobcreate000000000", "--wait", "--output", "yaml")
	require.NoError(t, err)
	var final map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(result.Stdout), &final))
	assert.Equal(t, "succeeded", final["status"])
	assert.NotContains(t, result.Stdout, "Waiting for job")
}

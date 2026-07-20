package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/client"
	telemetryclient "github.com/render-oss/cli/pkg/client/clitelemetry"
	"github.com/render-oss/cli/pkg/command"
)

func TestSenderSendAndLogGates(t *testing.T) {
	wantPayload := client.CreateCliTelemetryEventJSONRequestBody{
		Arch:           "test-arch",
		CliVersion:     "v-test",
		Command:        "render services list",
		CompletionKind: telemetryclient.ExplicitExit,
		DurationMs:     125,
		ExitCode:       7,
		Os:             "test-os",
	}
	payloadJSON, err := json.Marshal(wantPayload)
	require.NoError(t, err)
	payloadLog := string(payloadJSON) + "\n"

	type sendResult struct {
		requestCount int
		bodyClosed   bool
		payload      client.CreateCliTelemetryEventJSONRequestBody
		logOutput    string
	}
	testCases := []struct {
		name       string
		shouldSend bool
		shouldLog  bool
		want       sendResult
	}{
		{name: "disabled"},
		{
			name:      "logging alone",
			shouldLog: true,
			want:      sendResult{logOutput: payloadLog},
		},
		{
			name:       "send silently",
			shouldSend: true,
			want: sendResult{
				requestCount: 1,
				bodyClosed:   true,
				payload:      wantPayload,
			},
		},
		{
			name:       "send and log",
			shouldSend: true,
			shouldLog:  true,
			want: sendResult{
				requestCount: 1,
				bodyClosed:   true,
				payload:      wantPayload,
				logOutput:    payloadLog + "analytics response: 202 Accepted\n",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := &eventPOSTResponseBody{}
			apiClient := &fakeTelemetryClient{
				response: &http.Response{StatusCode: http.StatusAccepted, Status: "202 Accepted", Body: body},
			}
			sender := newTestSender(apiClient, tc.shouldSend, tc.shouldLog)
			var stderr bytes.Buffer

			sender.Send(command.ExecutionResult{
				CommandPath:    "render services list",
				CompletionKind: command.CompletionKindExplicitExit,
				Duration:       125 * time.Millisecond,
				ExitCode:       7,
			}, &stderr)

			require.Equal(t, tc.want, sendResult{
				requestCount: apiClient.calls,
				bodyClosed:   body.closed,
				payload:      apiClient.payload,
				logOutput:    stderr.String(),
			})
		})
	}
}

func TestNewUsesExactEnvironmentGates(t *testing.T) {
	testCases := []struct {
		name           string
		sendValue      string
		logValue       string
		wantShouldSend bool
		wantShouldLog  bool
	}{
		{name: "unset"},
		{name: "non-one values", sendValue: "true", logValue: "true"},
		{name: "logging only", logValue: "1", wantShouldLog: true},
		{name: "sending only", sendValue: "1", wantShouldSend: true},
		{name: "both", sendValue: "1", logValue: "1", wantShouldSend: true, wantShouldLog: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RENDER_TEST_ENABLE_ANALYTICS", tc.sendValue)
			t.Setenv("RENDER_LOG_ANALYTICS", tc.logValue)

			sender := New(&client.ClientWithResponses{})

			require.Equal(t, tc.wantShouldSend, sender.shouldSend)
			require.Equal(t, tc.wantShouldLog, sender.shouldLog)
		})
	}
}

func TestSenderUsesConfiguredAPIClient(t *testing.T) {
	server := renderapi.NewServer(t)
	t.Setenv("RENDER_TEST_ENABLE_ANALYTICS", "1")
	t.Setenv("RENDER_HOST", server.URL()+"/")
	t.Setenv("RENDER_API_KEY", "secret-token")
	apiClient, err := client.NewDefaultClient()
	require.NoError(t, err)
	sender := New(apiClient)

	sender.Send(command.ExecutionResult{
		CommandPath:    "render services list",
		CompletionKind: command.CompletionKindSuccess,
		ExitCode:       0,
	}, io.Discard)

	require.Equal(t, []client.CreateCliTelemetryEventJSONRequestBody{{
		Arch:           runtime.GOARCH,
		CliVersion:     cfg.Version,
		Command:        "render services list",
		CompletionKind: telemetryclient.Success,
		ExitCode:       0,
		Os:             runtime.GOOS,
	}}, server.CliTelemetry.Instances)
}

// TestCompletionKindsAreKnownToTelemetryAPI guards against local/remote enum
// drift: Send casts a command.CompletionKind straight to the generated
// telemetry enum without validating it. If a new local completion kind is added
// before the API schema knows about it, this test fails until the client is
// regenerated, rather than the CLI silently sending an unknown value.
//
// I am considering getting rid of the server-owned enum, at which point we could delete this test
// The server could simply expect a string here instead, allowing client to own the completion kinds fully
func TestCompletionKindsAreKnownToTelemetryAPI(t *testing.T) {
	for _, kind := range command.CompletionKindValues() {
		t.Run(string(kind), func(t *testing.T) {
			apiKind := telemetryclient.CliTelemetryEventPOSTInputCompletionKind(kind)
			require.Truef(t, apiKind.Valid(),
				"completion kind %q is not a known telemetry enum value; add it to the API schema and regenerate the client", kind)
		})
	}
}

func TestSenderCancelsSendAfterTimeout(t *testing.T) {
	type requestObservation struct {
		ctx         context.Context
		deadline    time.Time
		observedAt  time.Time
		hasDeadline bool
	}

	apiClient := &fakeTelemetryClient{}
	requestObserved := make(chan requestObservation, 1)
	releaseRequest := make(chan struct{})
	defer close(releaseRequest)
	apiClient.handler = func(ctx context.Context, _ client.CreateCliTelemetryEventJSONRequestBody) (*http.Response, error) {
		deadline, hasDeadline := ctx.Deadline()
		requestObserved <- requestObservation{
			ctx:         ctx,
			deadline:    deadline,
			observedAt:  time.Now(),
			hasDeadline: hasDeadline,
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-releaseRequest:
			return nil, errors.New("test released blocking request")
		}
	}
	sender := newTestSender(apiClient, true, true)
	var stderr bytes.Buffer
	sendDone := make(chan struct{})
	startedAt := time.Now()

	go func() {
		sender.Send(command.ExecutionResult{CommandPath: "render services list"}, &stderr)
		close(sendDone)
	}()

	var observation requestObservation
	select {
	case observation = <-requestObserved:
	case <-time.After(time.Second):
		t.Fatal("analytics request did not start")
	}

	select {
	case <-sendDone:
	case <-time.After(time.Second):
		t.Fatal("Sender.Send did not return after the analytics timeout")
	}

	require.Equal(t, 1, apiClient.calls)
	require.True(t, observation.hasDeadline, "analytics requests should have a hard deadline")
	deadlineRemaining := observation.deadline.Sub(observation.observedAt)
	require.LessOrEqual(t, deadlineRemaining, sendTimeout,
		"request deadline should not exceed the configured timeout: remaining=%s timeout=%s", deadlineRemaining, sendTimeout)
	elapsed := time.Since(startedAt)
	require.GreaterOrEqual(t, elapsed, sendTimeout,
		"blocking request should not be canceled before its deadline: elapsed=%s timeout=%s", elapsed, sendTimeout)
	require.ErrorIs(t, observation.ctx.Err(), context.DeadlineExceeded,
		"blocking analytics request should be canceled because its deadline expired")
	require.Contains(t, stderr.String(), "analytics error: canceling API request after exceeding 250ms timeout")
}

func TestAnalyticsLogWriteFailureDoesNotPreventRequestOrCleanup(t *testing.T) {
	body := &eventPOSTResponseBody{}
	apiClient := &fakeTelemetryClient{
		response: &http.Response{StatusCode: http.StatusAccepted, Status: "202 Accepted", Body: body},
	}
	sender := newTestSender(apiClient, true, true)

	sender.Send(command.ExecutionResult{CommandPath: "render services list"}, failingWriter{})

	require.Equal(t, 1, apiClient.calls)
	require.True(t, body.closed)
}

func TestHTTPFailureIsSwallowedWithoutRetry(t *testing.T) {
	runFailedSend := func(t *testing.T, shouldLog bool) string {
		t.Helper()

		body := &eventPOSTResponseBody{}
		apiClient := &fakeTelemetryClient{
			response: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Status:     "500 Internal Server Error",
				Body:       body,
			},
		}
		sender := newTestSender(apiClient, true, shouldLog)
		var stderr bytes.Buffer

		sender.Send(command.ExecutionResult{CommandPath: "render services list"}, &stderr)

		require.Equal(t, 1, apiClient.calls)
		require.True(t, body.closed)
		return stderr.String()
	}

	t.Run("logging enabled", func(t *testing.T) {
		logOutput := runFailedSend(t, true)
		require.Contains(t, logOutput, "analytics response: 500 Internal Server Error")
	})

	t.Run("logging disabled", func(t *testing.T) {
		logOutput := runFailedSend(t, false)
		require.Empty(t, logOutput)
	})
}

// newTestSender builds a Sender with fixed environment fields so tests exercise
// the gates and transport without depending on the host's cfg/runtime values.
func newTestSender(apiClient cliTelemetryClient, shouldSend, shouldLog bool) *Sender {
	return &Sender{
		client:     apiClient,
		shouldSend: shouldSend,
		shouldLog:  shouldLog,
		cliVersion: "v-test",
		goos:       "test-os",
		goarch:     "test-arch",
		timeout:    sendTimeout,
	}
}

type fakeTelemetryClient struct {
	calls    int
	payload  client.CreateCliTelemetryEventJSONRequestBody
	response *http.Response
	err      error
	handler  func(context.Context, client.CreateCliTelemetryEventJSONRequestBody) (*http.Response, error)
}

func (f *fakeTelemetryClient) CreateCliTelemetryEvent(ctx context.Context, payload client.CreateCliTelemetryEventJSONRequestBody, _ ...client.RequestEditorFn) (*http.Response, error) {
	f.calls++
	f.payload = payload
	if f.handler != nil {
		return f.handler(ctx, payload)
	}
	return f.response, f.err
}

type eventPOSTResponseBody struct {
	closed bool
}

func (b *eventPOSTResponseBody) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (b *eventPOSTResponseBody) Close() error {
	b.closed = true
	return nil
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

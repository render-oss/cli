package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/client"
	telemetryclient "github.com/render-oss/cli/pkg/client/clitelemetry"
	"github.com/render-oss/cli/pkg/command"
)

const sendTimeout = 250 * time.Millisecond

// Sender turns a completed execution into an analytics event. Sending and
// logging events are controlled independently by environment variables.
//
// New configures sending only when analytics sending is enabled and the API
// client is present. A logged-out CLI still holds a client
// (client.NotLoggedInClient), whose request editor fails in-process before any
// network I/O, so sending through it is safe. Event logging works even when
// sending is disabled.
type Sender struct {
	// client sends analytics events to the Render API.
	client cliTelemetryClient
	// shouldSend controls whether analytics events are sent.
	shouldSend bool
	// shouldLog controls whether analytics activity is logged.
	shouldLog bool
	// cliVersion is the Render CLI version included in each event.
	cliVersion string
	// goos is the operating system included in each event.
	goos string
	// goarch is the architecture included in each event.
	goarch string
	// timeout limits how long an analytics request may take.
	timeout time.Duration
}

// New creates a new [Sender].
func New(apiClient *client.ClientWithResponses) *Sender {
	return &Sender{
		client:     apiClient,
		shouldSend: cfg.ShouldSendAnalytics() && apiClient != nil,
		shouldLog:  cfg.ShouldLogAnalytics(),
		cliVersion: cfg.Version,
		goos:       runtime.GOOS,
		goarch:     runtime.GOARCH,
		timeout:    sendTimeout,
	}
}

// Send builds the analytics event for a completed execution and dispatches it
// after Cobra and its post-run hooks finish. It never returns errors to the
// command path: sending is best-effort, and failures are only surfaced through
// logging.
//
// In the future if we have more than 1 event type to send, we can rename / refactor this function
func (s *Sender) Send(result command.ExecutionResult, stderr io.Writer) {
	if !s.shouldSend && !s.shouldLog {
		return
	}

	payload := client.CreateCliTelemetryEventJSONRequestBody{
		Arch:           s.goarch,
		CliVersion:     s.cliVersion,
		Command:        result.CommandPath,
		CompletionKind: telemetryclient.CliTelemetryEventPOSTInputCompletionKind(result.CompletionKind),
		DurationMs:     result.Duration.Milliseconds(),
		ExitCode:       result.ExitCode,
		Os:             s.goos,
	}

	if s.shouldLog {
		payloadJSON, _ := json.Marshal(payload)
		_, _ = fmt.Fprintln(stderr, string(payloadJSON))
	}

	if !s.shouldSend {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	response, err := s.client.CreateCliTelemetryEvent(ctx, payload)
	if response != nil && response.Body != nil {
		defer func() { _ = response.Body.Close() }()
	}

	if !s.shouldLog {
		return
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			_, _ = fmt.Fprintf(stderr, "analytics error: canceling API request after exceeding %s timeout\n", s.timeout)
			return
		}
		_, _ = fmt.Fprintf(stderr, "analytics error: %v\n", err)
		return
	}
	if response == nil {
		_, _ = fmt.Fprintf(stderr, "analytics error: %v\n", errors.New("empty response"))
		return
	}
	_, _ = fmt.Fprintf(stderr, "analytics response: %s\n", response.Status)
}

// cliTelemetryClient is the slice of the generated API client that Sender needs.
// The API resource is a "cli telemetry event"; that wire vocabulary stays at
// this boundary while everything the CLI owns speaks in terms of analytics.
type cliTelemetryClient interface {
	CreateCliTelemetryEvent(ctx context.Context, body client.CreateCliTelemetryEventJSONRequestBody, reqEditors ...client.RequestEditorFn) (*http.Response, error)
}

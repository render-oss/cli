package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/render-oss/cli/internal/installid"
	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/client"
	telemetryclient "github.com/render-oss/cli/pkg/client/clitelemetry"
	"github.com/render-oss/cli/pkg/command"
)

// unknownOutputFormat identifies invocations that finished before command setup
// resolved an output format.
const unknownOutputFormat = "unknown"

// Sender turns a completed execution into an analytics event. Sending and
// logging are controlled independently: sending by the dev gate and
// [ResolveConsent] (env opt-outs and the config file), logging by an
// environment variable.
//
// New configures sending only when the dev gate is open, the user has not
// opted out, and the API client is present. A logged-out CLI still holds a
// client
// (client.NotLoggedInClient), whose request editor fails in-process before any
// network I/O, so sending through it is safe. Event logging works even when
// sending is disabled.
type Sender struct {
	// client sends analytics events to the Render API.
	client cliTelemetryClient
	// sendingEnabled controls whether analytics events may be sent based on the
	// dev gate, user consent, and client presence.
	sendingEnabled bool
	// shouldLog controls whether analytics activity is logged.
	shouldLog bool
	// cliVersion is the Render CLI version included in each event.
	cliVersion string
	// goos is the operating system included in each event.
	goos string
	// goarch is the architecture included in each event.
	goarch string
	// detectTerminalSignals observes terminal attachment and TERM=dumb.
	detectTerminalSignals func() command.TerminalSignals
	// detectAgentSignals observes allowlisted agent environment markers.
	detectAgentSignals func() []string
	// detectCISignals observes allow-listed automated pipeline markers.
	detectCISignals func() []string
	// getInstallationID resolves the stable identifier included in each event.
	getInstallationID func() (string, error)
	// newLauncher lazily resolves the subprocess used to send an event file.
	newLauncher func() (analyticsSubprocessLauncher, error)
}

type analyticsSubprocessLauncher interface {
	startDetached(eventFile string) error
	runSync(ctx context.Context, eventFile string, stderr io.Writer) error
}

// New creates a new [Sender].
func New(apiClient *client.ClientWithResponses) *Sender {
	// sendingEnabled is a conjunction of the internal dev gate and user consent.
	// The conjunction is what lets the opt-out consent logic ship without
	// being activated: sending still requires the dev gate to be explicitly
	// opened, so analytics stays opt-in by default — while a dev with the
	// gate open and any opt-out mechanism set sends nothing, so the consent
	// path is live end to end before the default ever changes.
	//
	// Deleting the cfg.AnalyticsDevGateOpen() term is the change that flips
	// the CLI from opt-in to opt-out (GROW-3146), leaving user consent and
	// client presence as the only conditions. Until then, order matters: the
	// dev gate short-circuits first, so builds with the gate closed never
	// read the config file that consent lives in.
	sendingEnabled := cfg.AnalyticsDevGateOpen() && ResolveConsent().Granted && apiClient != nil

	return newSender(
		apiClient,
		sendingEnabled,
		cfg.ShouldLogAnalytics(),
		command.DetectTerminalSignals,
		DetectAgentSignals,
		DetectCISignals,
	)
}

func newSender(
	apiClient cliTelemetryClient,
	sendingEnabled bool,
	shouldLog bool,
	detectTerminalSignals func() command.TerminalSignals,
	detectAgentSignals func() []string,
	detectCISignals func() []string,
) *Sender {
	return &Sender{
		client:                apiClient,
		sendingEnabled:        sendingEnabled,
		shouldLog:             shouldLog,
		cliVersion:            cfg.Version,
		goos:                  runtime.GOOS,
		goarch:                runtime.GOARCH,
		detectTerminalSignals: detectTerminalSignals,
		detectAgentSignals:    detectAgentSignals,
		detectCISignals:       detectCISignals,
		getInstallationID:     installid.Resolve,
		newLauncher: func() (analyticsSubprocessLauncher, error) {
			return newSubprocessLauncher()
		},
	}
}

// Send builds the analytics event for a completed execution, writes it to a
// state file, and hands that file to a subprocess after Cobra and its post-run
// hooks finish. It never performs network I/O in the calling process or returns
// errors to the command path: sending is best-effort, and failures are only
// surfaced through logging.
//
// In the future if we have more than 1 event type to send, we can rename / refactor this function
func (s *Sender) Send(result command.ExecutionResult, stderr io.Writer) {
	if !s.sendingEnabled && !s.shouldLog {
		return
	}

	now := time.Now()
	var currentBackoff backoff
	if s.sendingEnabled {
		currentBackoff = loadBackoff()
	}
	backingOff := currentBackoff.inEffect(now)
	if backingOff && !s.shouldLog {
		return
	}
	shouldSend := s.sendingEnabled && !backingOff

	var launcher analyticsSubprocessLauncher
	var launcherErr error
	if shouldSend {
		// A silent refusal has nothing to report, so stop before resolving the
		// installation ID or creating analytics state.
		launcher, launcherErr = s.newLauncher()
		if launcherErr != nil && !s.shouldLog {
			return
		}
	}

	installationID, err := s.getInstallationID()
	if err != nil {
		s.logError(stderr, fmt.Errorf("getting installation ID: %w", err))
	}

	terminalSignals := s.detectTerminalSignals()
	agentSignals := s.detectAgentSignals()
	ciSignals := s.detectCISignals()
	payload := newEventPOSTBody(result, terminalSignals, agentSignals, ciSignals, installationID, s.cliVersion, s.goos, s.goarch)

	if s.shouldLog {
		payloadJSON, _ := json.Marshal(payload)
		_, _ = fmt.Fprintln(stderr, string(payloadJSON))
	}

	if !shouldSend {
		if backingOff && s.shouldLog {
			_, _ = fmt.Fprintln(stderr, currentBackoff.skippedSendDiagnostic())
		}
		return
	}

	if launcherErr != nil {
		_, diagnostic := resolveStrategy(strategyInputs{
			isCI:               cfg.IsCI(),
			loggingEnabled:     s.shouldLog,
			configuredStrategy: cfg.AnalyticsStrategy(),
		})
		if diagnostic != "" {
			_, _ = fmt.Fprintln(stderr, diagnostic)
		}
		s.logError(stderr, launcherErr)
		return
	}

	eventFile, err := writeEventFile(payload)
	if err != nil {
		s.logError(stderr, err)
		return
	}

	strategy, diagnostic := resolveStrategy(strategyInputs{
		isCI:               cfg.IsCI(),
		loggingEnabled:     s.shouldLog,
		configuredStrategy: cfg.AnalyticsStrategy(),
	})
	if diagnostic != "" && s.shouldLog {
		_, _ = fmt.Fprintln(stderr, diagnostic)
	}

	//exhaustive:enforce
	switch strategy {
	case strategyDetached:
		err = launcher.startDetached(eventFile)
	case strategySync:
		diagnostics := io.Discard
		if s.shouldLog {
			diagnostics = stderr
		}
		err = launcher.runSync(context.Background(), eventFile, diagnostics)
	default:
		// A strategy this switch does not handle would otherwise leave err nil
		// and silently orphan the event file.
		err = fmt.Errorf("unhandled analytics send strategy %d", strategy)
	}
	if err != nil {
		// The child normally removes the file before sending. If it failed before
		// taking ownership, the parent drops the event instead of orphaning it.
		_ = os.Remove(eventFile)
		s.logError(stderr, err)
	}
}

func (s *Sender) logError(stderr io.Writer, err error) {
	if s.shouldLog {
		_, _ = fmt.Fprintf(stderr, "analytics error: %v\n", err)
	}
}

func newEventPOSTBody(
	result command.ExecutionResult,
	terminalSignals command.TerminalSignals,
	agentSignals []string,
	ciSignals []string,
	installationID string,
	cliVersion string,
	goos string,
	goarch string,
) client.CreateCliTelemetryEventJSONRequestBody {
	return client.CreateCliTelemetryEventJSONRequestBody{
		AgentSignals:          agentSignals,
		Arch:                  goarch,
		CiSignals:             ciSignals,
		CliVersion:            cliVersion,
		Command:               result.CommandPath,
		CompletionKind:        telemetryclient.CliTelemetryEventPOSTInputCompletionKind(result.CompletionKind),
		DurationMs:            result.Duration.Milliseconds(),
		ExitCode:              result.ExitCode,
		IsStdinTty:            terminalSignals.StdinTTY,
		IsStdoutTty:           terminalSignals.StdoutTTY,
		IsStderrTty:           terminalSignals.StderrTTY,
		IsTermDumb:            terminalSignals.DumbTerminal,
		InstallationId:        installationID,
		LaunchedFullScreenTui: &result.LaunchedFullScreenTUI,
		Os:                    goos,
		OutputFormat:          analyticsOutputFormat(result.OutputFormat),
		StartedAt:             result.StartedAt.UTC().Format(time.RFC3339),
	}
}

func analyticsOutputFormat(output *command.Output) string {
	if output == nil {
		return unknownOutputFormat
	}
	return string(*output)
}

// cliTelemetryClient is the slice of the generated API client that Sender needs.
// The API resource is a "cli telemetry event"; that wire vocabulary stays at
// this boundary while everything the CLI owns speaks in terms of analytics.
type cliTelemetryClient interface {
	CreateCliTelemetryEvent(ctx context.Context, body client.CreateCliTelemetryEventJSONRequestBody, reqEditors ...client.RequestEditorFn) (*http.Response, error)
}

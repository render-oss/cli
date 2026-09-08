package job

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/render-oss/cli/pkg/client"
	clientjob "github.com/render-oss/cli/pkg/client/jobs"
	logclient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/render-oss/cli/pkg/logs"
)

const (
	DefaultPollInterval      = 2 * time.Second
	DefaultTimeout           = 30 * time.Minute
	DefaultTransientAttempts = 3
)

// TerminalError reports a completed job whose status should fail a CI step.
// The final Job remains available so the command can print it before returning
// the nonzero result through Cobra.
type TerminalError struct {
	Job *clientjob.Job
}

func (e *TerminalError) Error() string {
	return fmt.Sprintf("job %s finished with status %s", e.Job.Id, statusName(e.Job.Status))
}

// StatusError reports a missing or unrecognized job status. Treating these as
// errors prevents an API/schema mismatch from producing an infinite wait.
type StatusError struct {
	JobID  string
	Status *clientjob.JobStatus
}

func (e *StatusError) Error() string {
	if e.Status == nil {
		return fmt.Sprintf("job %s returned no status", e.JobID)
	}
	return fmt.Sprintf("job %s returned unknown status %q", e.JobID, *e.Status)
}

// TickSource is the small portion of time.Ticker needed by Runner. Tests use a
// manually driven implementation, so state transitions never require sleeps.
type TickSource interface {
	C() <-chan time.Time
	Stop()
}

type realTicker struct{ ticker *time.Ticker }

func (t realTicker) C() <-chan time.Time { return t.ticker.C }
func (t realTicker) Stop()               { t.ticker.Stop() }

// OpenLogStream opens a read-only stream beginning at startTime. A reconnect
// receives the last observed timestamp and relies on log ID deduplication to
// handle inclusive server cursors.
type OpenLogStream func(ctx context.Context, startTime *time.Time) (*logs.TailStream, error)

// Runner coordinates job status polling with the existing live log stream.
type Runner struct {
	Retrieve             func(context.Context) (*clientjob.Job, error)
	OpenLogs             OpenLogStream
	PollInterval         time.Duration
	MaxTransientAttempts int
	NewTicker            func(time.Duration) TickSource
	OnLog                func(*logclient.Log) error
	OnRetry              func(string)
}

// Run waits for a terminal status. tail controls whether the existing log
// stream is opened alongside status polling.
func (r Runner) Run(ctx context.Context, jobID string, tail bool, timeout time.Duration) (*clientjob.Job, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("timeout must be positive")
	}
	if r.Retrieve == nil {
		return nil, errors.New("job status retriever is required")
	}
	if tail && r.OpenLogs == nil {
		return nil, errors.New("job log stream is required when tailing")
	}

	pollInterval := r.PollInterval
	if pollInterval <= 0 {
		pollInterval = DefaultPollInterval
	}
	maxTransientAttempts := r.MaxTransientAttempts
	if maxTransientAttempts <= 0 {
		maxTransientAttempts = DefaultTransientAttempts
	}
	newTicker := r.NewTicker
	if newTicker == nil {
		newTicker = func(interval time.Duration) TickSource {
			return realTicker{ticker: time.NewTicker(interval)}
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := newTicker(pollInterval)
	defer ticker.Stop()

	seenLogs := make(map[string]struct{})
	var lastLogTime *time.Time
	var logStream *logs.TailStream
	var stopLogs context.CancelFunc
	var reconnectLogs bool
	var allowReconnect bool
	var statusFailures int
	var streamFailures int

	closeLogStream := func() {
		if stopLogs != nil {
			stopLogs()
			stopLogs = nil
		}
	}
	defer closeLogStream()

	openLogs := func() error {
		streamCtx, streamCancel := context.WithCancel(runCtx)
		stream, err := r.OpenLogs(streamCtx, lastLogTime)
		if err != nil {
			streamCancel()
			return err
		}
		closeLogStream()
		stopLogs = streamCancel
		logStream = stream
		reconnectLogs = false
		return nil
	}

	if tail {
		if err := openLogs(); err != nil {
			if !logs.IsTransientStreamError(err) {
				return nil, err
			}
			streamFailures++
			reconnectLogs = true
			r.reportRetry(fmt.Sprintf("log stream failed; retrying (%d/%d): %v", streamFailures, maxTransientAttempts, err))
		}
	}

	poll := true
	for {
		if tail && reconnectLogs && allowReconnect {
			allowReconnect = false
			if streamFailures > maxTransientAttempts {
				return nil, fmt.Errorf("tail logs for job %s: transient failure persisted after %d retries", jobID, maxTransientAttempts)
			}
			if err := openLogs(); err != nil {
				if !logs.IsTransientStreamError(err) {
					return nil, err
				}
				streamFailures++
				if streamFailures > maxTransientAttempts {
					return nil, fmt.Errorf("tail logs for job %s: transient failure persisted after %d retries: %w", jobID, maxTransientAttempts, err)
				}
				r.reportRetry(fmt.Sprintf("log stream failed; retrying (%d/%d): %v", streamFailures, maxTransientAttempts, err))
			} else {
				streamFailures = 0
			}
		}

		if poll {
			current, err := r.Retrieve(runCtx)
			if err != nil {
				if runCtx.Err() != nil {
					return nil, waitContextError(runCtx, jobID, timeout)
				}
				if !isTransientRetrieveError(err) {
					return nil, fmt.Errorf("retrieve job %s: %w", jobID, err)
				}
				statusFailures++
				if statusFailures > maxTransientAttempts {
					return nil, fmt.Errorf("retrieve job %s: transient failure persisted after %d retries: %w", jobID, maxTransientAttempts, err)
				}
				r.reportRetry(fmt.Sprintf("job status failed; retrying (%d/%d): %v", statusFailures, maxTransientAttempts, err))
			} else {
				statusFailures = 0
				done, terminalErr := terminalResult(current)
				if done {
					closeLogStream()
					if err := r.drainBufferedLogs(logStream, seenLogs, &lastLogTime); err != nil {
						return current, err
					}
					return current, terminalErr
				}
			}
			poll = false
		}

		var logCh <-chan *logclient.Log
		var logErrCh <-chan error
		if logStream != nil {
			logCh = logStream.Logs
			logErrCh = logStream.Errors
		}

		select {
		case <-runCtx.Done():
			closeLogStream()
			if err := r.drainBufferedLogs(logStream, seenLogs, &lastLogTime); err != nil {
				return nil, err
			}
			return nil, waitContextError(runCtx, jobID, timeout)
		case <-ticker.C():
			poll = true
			allowReconnect = true
		case entry, ok := <-logCh:
			if !ok {
				if logStream != nil {
					logStream.Logs = nil
				}
				continue
			}
			if err := r.emitLog(entry, seenLogs, &lastLogTime); err != nil {
				return nil, err
			}
		case streamErr, ok := <-logErrCh:
			if !ok {
				if logStream != nil {
					logStream.Errors = nil
				}
				continue
			}
			closeLogStream()
			if err := r.drainBufferedLogs(logStream, seenLogs, &lastLogTime); err != nil {
				return nil, err
			}
			logStream = nil
			if !logs.IsTransientStreamError(streamErr) {
				return nil, fmt.Errorf("tail logs for job %s: %w", jobID, streamErr)
			}
			streamFailures++
			reconnectLogs = true
			r.reportRetry(fmt.Sprintf("log stream disconnected; retrying (%d/%d): %v", streamFailures, maxTransientAttempts, streamErr))
		}
	}
}

func waitContextError(ctx context.Context, jobID string, timeout time.Duration) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("timed out waiting for job %s after %s", jobID, timeout)
	}
	return fmt.Errorf("waiting for job %s canceled: %w", jobID, ctx.Err())
}

func (r Runner) emitLog(entry *logclient.Log, seen map[string]struct{}, lastLogTime **time.Time) error {
	if entry == nil {
		return nil
	}
	key := entry.Id
	if key == "" {
		key = entry.Timestamp.UTC().Format(time.RFC3339Nano) + "\x00" + entry.Message
	}
	if _, exists := seen[key]; exists {
		return nil
	}
	seen[key] = struct{}{}
	timestamp := entry.Timestamp
	*lastLogTime = &timestamp
	if r.OnLog != nil {
		return r.OnLog(entry)
	}
	return nil
}

func (r Runner) drainBufferedLogs(stream *logs.TailStream, seen map[string]struct{}, lastLogTime **time.Time) error {
	if stream == nil {
		return nil
	}
	for stream.Logs != nil {
		select {
		case entry, ok := <-stream.Logs:
			if !ok {
				stream.Logs = nil
				continue
			}
			if err := r.emitLog(entry, seen, lastLogTime); err != nil {
				return err
			}
		default:
			return nil
		}
	}
	return nil
}

func (r Runner) reportRetry(message string) {
	if r.OnRetry != nil {
		r.OnRetry(message)
	}
}

func terminalResult(current *clientjob.Job) (bool, error) {
	if current == nil {
		return true, errors.New("retrieve job returned an empty response")
	}
	if current.Status == nil {
		return true, &StatusError{JobID: current.Id}
	}
	switch *current.Status {
	case clientjob.Pending, clientjob.Running:
		return false, nil
	case clientjob.Succeeded:
		return true, nil
	case clientjob.Failed, clientjob.Canceled:
		return true, &TerminalError{Job: current}
	default:
		return true, &StatusError{JobID: current.Id, Status: current.Status}
	}
}

func statusName(status *clientjob.JobStatus) string {
	if status == nil {
		return "unknown"
	}
	return string(*status)
}

func isTransientRetrieveError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, client.ErrTooManyRequests) {
		return true
	}
	var responseErr *HTTPError
	if errors.As(err, &responseErr) {
		return responseErr.StatusCode == http.StatusTooManyRequests || responseErr.StatusCode >= http.StatusInternalServerError
	}
	var netErr net.Error
	return (errors.As(err, &netErr) && netErr.Timeout()) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNABORTED)
}

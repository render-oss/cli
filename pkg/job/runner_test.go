package job

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clientjob "github.com/render-oss/cli/pkg/client/jobs"
	logclient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/render-oss/cli/pkg/logs"
	"github.com/render-oss/cli/pkg/pointers"
)

type manualTicker struct{ ticks chan time.Time }

func newManualTicker(buffer int) *manualTicker {
	return &manualTicker{ticks: make(chan time.Time, buffer)}
}

func (t *manualTicker) C() <-chan time.Time { return t.ticks }
func (t *manualTicker) Stop()               {}
func (t *manualTicker) Tick()               { t.ticks <- time.Time{} }

func jobWithStatus(status *clientjob.JobStatus) *clientjob.Job {
	return &clientjob.Job{Id: "job-test", Status: status}
}

func sequenceRetriever(results ...*clientjob.Job) func(context.Context) (*clientjob.Job, error) {
	var mu sync.Mutex
	index := 0
	return func(context.Context) (*clientjob.Job, error) {
		mu.Lock()
		defer mu.Unlock()
		if index >= len(results) {
			return results[len(results)-1], nil
		}
		result := results[index]
		index++
		return result, nil
	}
}

func TestRunnerWaitsForSucceededJobWithoutSleeping(t *testing.T) {
	ticker := newManualTicker(2)
	ticker.Tick()
	ticker.Tick()
	runner := Runner{
		Retrieve: sequenceRetriever(
			jobWithStatus(pointers.From(clientjob.Pending)),
			jobWithStatus(pointers.From(clientjob.Running)),
			jobWithStatus(pointers.From(clientjob.Succeeded)),
		),
		NewTicker: func(time.Duration) TickSource { return ticker },
	}

	result, err := runner.Run(context.Background(), "job-test", false, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, result.Status)
	assert.Equal(t, clientjob.Succeeded, *result.Status)
}

func TestRunnerReturnsTerminalErrors(t *testing.T) {
	for _, status := range []clientjob.JobStatus{clientjob.Failed, clientjob.Canceled} {
		t.Run(string(status), func(t *testing.T) {
			runner := Runner{Retrieve: sequenceRetriever(jobWithStatus(pointers.From(status)))}
			result, err := runner.Run(context.Background(), "job-test", false, time.Minute)
			var terminalErr *TerminalError
			require.ErrorAs(t, err, &terminalErr)
			assert.Same(t, result, terminalErr.Job)
		})
	}
}

func TestRunnerRejectsMissingAndUnknownStatuses(t *testing.T) {
	unknown := clientjob.JobStatus("paused")
	for name, status := range map[string]*clientjob.JobStatus{"missing": nil, "unknown": &unknown} {
		t.Run(name, func(t *testing.T) {
			runner := Runner{Retrieve: sequenceRetriever(jobWithStatus(status))}
			_, err := runner.Run(context.Background(), "job-test", false, time.Minute)
			var statusErr *StatusError
			require.ErrorAs(t, err, &statusErr)
		})
	}
}

func TestRunnerRetriesBoundedTransientRetrieveFailure(t *testing.T) {
	ticker := newManualTicker(1)
	ticker.Tick()
	calls := 0
	runner := Runner{
		Retrieve: func(context.Context) (*clientjob.Job, error) {
			calls++
			if calls == 1 {
				return nil, &HTTPError{StatusCode: http.StatusServiceUnavailable, Err: errors.New("try later")}
			}
			return jobWithStatus(pointers.From(clientjob.Succeeded)), nil
		},
		NewTicker: func(time.Duration) TickSource { return ticker },
	}

	_, err := runner.Run(context.Background(), "job-test", false, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
}

func TestRunnerRetriesTimeoutNetworkFailure(t *testing.T) {
	ticker := newManualTicker(1)
	ticker.Tick()
	calls := 0
	runner := Runner{
		Retrieve: func(context.Context) (*clientjob.Job, error) {
			calls++
			if calls == 1 {
				return nil, &net.DNSError{Err: "timeout", IsTimeout: true}
			}
			return jobWithStatus(pointers.From(clientjob.Succeeded)), nil
		},
		NewTicker: func(time.Duration) TickSource { return ticker },
	}

	_, err := runner.Run(context.Background(), "job-test", false, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
}

func TestRunnerStopsAfterTransientRetryLimit(t *testing.T) {
	ticker := newManualTicker(DefaultTransientAttempts)
	for range DefaultTransientAttempts {
		ticker.Tick()
	}
	calls := 0
	runner := Runner{
		Retrieve: func(context.Context) (*clientjob.Job, error) {
			calls++
			return nil, &HTTPError{StatusCode: http.StatusTooManyRequests, Err: errors.New("rate limited")}
		},
		NewTicker: func(time.Duration) TickSource { return ticker },
	}

	_, err := runner.Run(context.Background(), "job-test", false, time.Minute)
	require.ErrorContains(t, err, "persisted after 3 retries")
	assert.Equal(t, DefaultTransientAttempts+1, calls)
}

func TestRunnerStopsAfterTransientLogRetryLimit(t *testing.T) {
	ticker := newManualTicker(DefaultTransientAttempts)
	for range DefaultTransientAttempts {
		ticker.Tick()
	}
	openCalls := 0
	runner := Runner{
		Retrieve: sequenceRetriever(jobWithStatus(pointers.From(clientjob.Running))),
		OpenLogs: func(context.Context, *time.Time) (*logs.TailStream, error) {
			openCalls++
			return nil, &logs.StreamError{Err: errors.New("temporary disconnect"), Transient: true}
		},
		NewTicker: func(time.Duration) TickSource { return ticker },
	}

	_, err := runner.Run(context.Background(), "job-test", true, time.Minute)
	require.ErrorContains(t, err, "persisted after 3 retries")
	assert.Equal(t, DefaultTransientAttempts+1, openCalls)
}

func TestRunnerCancellationStopsWaiting(t *testing.T) {
	ticker := newManualTicker(0)
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	runner := Runner{
		Retrieve: func(context.Context) (*clientjob.Job, error) {
			close(started)
			return jobWithStatus(pointers.From(clientjob.Running)), nil
		},
		NewTicker: func(time.Duration) TickSource { return ticker },
	}

	result := make(chan error, 1)
	go func() {
		_, err := runner.Run(ctx, "job-test", false, time.Minute)
		result <- err
	}()
	<-started
	cancel()
	require.ErrorContains(t, <-result, "canceled")
}

func TestRunnerTimeoutStopsWaiting(t *testing.T) {
	runner := Runner{Retrieve: sequenceRetriever(jobWithStatus(pointers.From(clientjob.Running)))}
	_, err := runner.Run(context.Background(), "job-test", false, time.Nanosecond)
	require.ErrorContains(t, err, "timed out waiting for job job-test")
}

func TestRunnerCancellationAndTimeoutStopLogWorkers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		timeout time.Duration
		cancel  bool
	}{
		{name: "cancellation", timeout: time.Minute, cancel: true},
		{name: "timeout", timeout: time.Nanosecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			opened := make(chan struct{})
			stopped := make(chan struct{})
			runner := Runner{
				Retrieve: sequenceRetriever(jobWithStatus(pointers.From(clientjob.Running))),
				OpenLogs: func(streamCtx context.Context, _ *time.Time) (*logs.TailStream, error) {
					close(opened)
					go func() {
						<-streamCtx.Done()
						close(stopped)
					}()
					return &logs.TailStream{
						Logs:   make(chan *logclient.Log),
						Errors: make(chan error),
					}, nil
				},
			}
			result := make(chan error, 1)
			go func() {
				_, err := runner.Run(ctx, "job-test", true, tc.timeout)
				result <- err
			}()
			<-opened
			if tc.cancel {
				cancel()
			}
			require.Error(t, <-result)
			<-stopped
		})
	}
}

func TestRunnerReconnectsTailWithoutDuplicatingLogs(t *testing.T) {
	ticker := newManualTicker(0)
	firstLogs := make(chan *logclient.Log, 2)
	firstErrors := make(chan error, 1)
	secondLogs := make(chan *logclient.Log, 2)
	secondErrors := make(chan error)
	now := time.Unix(100, 0).UTC()
	firstLogs <- &logclient.Log{Id: "log-1", Message: "first", Timestamp: now}
	firstLogs <- &logclient.Log{Id: "log-2", Message: "second", Timestamp: now.Add(time.Second)}
	firstErrors <- &logs.StreamError{Err: errors.New("temporary disconnect"), Transient: true}

	opened := make(chan int, 2)
	openCalls := 0
	var resumedAt *time.Time
	streamClosed := make(chan struct{})
	runner := Runner{
		Retrieve: sequenceRetriever(
			jobWithStatus(pointers.From(clientjob.Running)),
			jobWithStatus(pointers.From(clientjob.Succeeded)),
		),
		OpenLogs: func(ctx context.Context, start *time.Time) (*logs.TailStream, error) {
			openCalls++
			if openCalls == 1 {
				opened <- openCalls
				return &logs.TailStream{Logs: firstLogs, Errors: firstErrors}, nil
			}
			resumedAt = start
			secondLogs <- &logclient.Log{Id: "log-2", Message: "second", Timestamp: now.Add(time.Second)}
			secondLogs <- &logclient.Log{Id: "log-3", Message: "third", Timestamp: now.Add(2 * time.Second)}
			go func() {
				<-ctx.Done()
				close(streamClosed)
			}()
			opened <- openCalls
			return &logs.TailStream{Logs: secondLogs, Errors: secondErrors}, nil
		},
		NewTicker: func(time.Duration) TickSource { return ticker },
	}

	received := make(chan string, 4)
	runner.OnLog = func(entry *logclient.Log) error {
		received <- entry.Message
		return nil
	}

	result := make(chan error, 1)
	go func() {
		_, err := runner.Run(context.Background(), "job-test", true, time.Minute)
		result <- err
	}()
	require.Equal(t, 1, <-opened)
	require.Equal(t, "first", <-received)
	require.Equal(t, "second", <-received)
	ticker.Tick() // permits the reconnect and the next status poll
	require.Equal(t, 2, <-opened)
	require.NotNil(t, resumedAt)
	assert.Equal(t, now.Add(time.Second), *resumedAt)

	require.Equal(t, "third", <-received)
	require.NoError(t, <-result)
	<-streamClosed
	assert.Equal(t, 2, openCalls)
	assert.Empty(t, received, fmt.Sprintf("unexpected duplicate log: %v", received))
}

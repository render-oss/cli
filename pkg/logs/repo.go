package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"

	"github.com/gorilla/websocket"

	"github.com/render-oss/cli/pkg/client"
	lclient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/render-oss/cli/pkg/config"
)

func NewLogRepo(c *client.ClientWithResponses, apiConfig *config.APIConfig) *LogRepo {
	return &LogRepo{c: c, apiConfig: apiConfig}
}

type LogRepo struct {
	c         *client.ClientWithResponses
	apiConfig *config.APIConfig
}

// TailStream reports logs and the reason a live stream ended. Errors is
// buffered so callers that only consume Logs (the legacy behavior) cannot
// prevent the reader goroutine from exiting.
type TailStream struct {
	Logs   <-chan *lclient.Log
	Errors <-chan error
}

// StreamError identifies whether reconnecting a log stream is safe. A stream
// reconnect only repeats a read; it never repeats the mutation that created the
// resource whose logs are being followed.
type StreamError struct {
	Err       error
	Transient bool
}

func (e *StreamError) Error() string { return e.Err.Error() }
func (e *StreamError) Unwrap() error { return e.Err }

// IsTransientStreamError reports whether a stream failure is safe to retry.
func IsTransientStreamError(err error) bool {
	var streamErr *StreamError
	return errors.As(err, &streamErr) && streamErr.Transient
}

func (l *LogRepo) ListLogs(ctx context.Context, params *client.ListLogsParams) (*client.Logs200Response, error) {
	logs, err := l.c.ListLogsWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(logs); err != nil {
		return nil, err
	}

	return logs.JSON200, nil
}

func (l *LogRepo) TailLogs(ctx context.Context, params *client.ListLogsParams) (<-chan *lclient.Log, error) {
	stream, err := l.TailLogsWithErrors(ctx, params)
	if err != nil {
		return nil, err
	}
	return stream.Logs, nil
}

// TailLogsWithErrors opens the same websocket used by TailLogs while exposing
// disconnect errors to callers that can safely reconnect.
func (l *LogRepo) TailLogsWithErrors(ctx context.Context, params *client.ListLogsParams) (*TailStream, error) {
	subscribeParams := client.SubscribeLogsParams(*params)
	req, err := client.NewSubscribeLogsRequest(l.apiConfig.Host, &subscribeParams)
	if err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{}

	u := req.URL

	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}

	// Establish WebSocket connection using the custom dialer
	conn, resp, err := dialer.DialContext(ctx, u.String(), client.AddHeaders(http.Header{}, l.apiConfig.Key))
	if err != nil {
		// Return the http error if it exists, fall back to the websocket error
		if resp != nil && resp.StatusCode != http.StatusSwitchingProtocols {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}

			streamErr := &StreamError{
				Err:       fmt.Errorf("failed to tail logs (HTTP %d): %s", resp.StatusCode, body),
				Transient: resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError,
			}
			return nil, streamErr
		}

		return nil, &StreamError{Err: err, Transient: isTransientStreamFailure(err)}
	}

	logs := make(chan *lclient.Log, 64)
	errs := make(chan error, 1)
	done := make(chan struct{})

	// A websocket read may block indefinitely. Closing the connection on
	// cancellation makes the read return and guarantees the worker can finish.
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	// Read messages from the WebSocket connection
	go func() {
		defer close(done)
		defer func() { _ = conn.Close() }()
		defer close(logs)
		defer close(errs)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if ctx.Err() == nil && !isNormalStreamClose(err) {
					errs <- &StreamError{Err: err, Transient: isTransientStreamFailure(err)}
				}
				return
			}

			var log lclient.Log
			if err := json.Unmarshal(message, &log); err != nil {
				errs <- &StreamError{Err: fmt.Errorf("decode tailed log: %w", err), Transient: false}
				return
			}

			select {
			case logs <- &log:
			case <-ctx.Done():
				return
			}
		}
	}()

	return &TailStream{Logs: logs, Errors: errs}, nil
}

func isNormalStreamClose(err error) bool {
	return websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway)
}

func isTransientStreamFailure(err error) bool {
	if websocket.IsCloseError(err,
		websocket.CloseAbnormalClosure,
		websocket.CloseInternalServerErr,
		websocket.CloseServiceRestart,
		websocket.CloseTryAgainLater,
	) {
		return true
	}

	var netErr net.Error
	return (errors.As(err, &netErr) && netErr.Timeout()) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNABORTED)
}

package logs

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/render-oss/cli/pkg/client"
	lclient "github.com/render-oss/cli/pkg/client/logs"
)

// Event contains a log, a terminal error, or a connection-status update.
// Exactly one field is populated; the other fields have their zero values.
// Callers distinguish events by checking which field is populated.
type Event struct {
	Log    *lclient.Log
	Err    error
	Status string
}

// TailLogs subscribes to logs matching params over a WebSocket connection to
// GET /v1/logs/subscribe. It returns a channel of logs, connection-status updates,
// and terminal errors. Callers must cancel ctx when they stop consuming events.
// The same channel remains open across reconnection attempts.
//
// The tail runs in a background goroutine, so initial connection failures are
// also reported through the channel. Interrupted connections are retried with
// exponential backoff, resuming from the latest delivered timestamp with care
// taken to avoid showing the same log message more than once.
// Transient failures produce status updates; permanent failures produce an Err
// event and close the channel. Cancellation closes the channel; callers can
// inspect ctx.Err() for the reason.
func (l *LogRepo) TailLogs(ctx context.Context, params *client.ListLogsParams) <-chan Event {
	events := make(chan Event)
	session := &tailSession{
		repo:        l,
		events:      events,
		query:       *params,
		boundaryIDs: make(map[string]struct{}),
		delay:       time.Second,
	}
	go session.run(ctx)
	return events
}

// tailSession is one ongoing request to tail logs, spanning successive
// WebSocket connections and delivering events through a single channel.
type tailSession struct {
	// repo opens authenticated WebSocket connections.
	repo *LogRepo
	// events is the caller's channel for the lifetime of this tail.
	events chan Event
	// query contains the log filters, including the timestamp to resume from.
	query client.ListLogsParams
	// latest is the greatest timestamp among logs delivered to the caller.
	latest time.Time
	// boundaryIDs contains delivered log IDs with timestamp == latest.
	boundaryIDs map[string]struct{}
	// delay is the wait before the next connection attempt after a failure.
	delay time.Duration
}

// run attempts to connect and reads logs until cancellation or a permanent error.
// It keeps the event channel open while retrying and closes it when the tail ends.
// Each iteration attempts to open one connection and, if successful, reads it
// before deciding whether to stop or wait and retry. readLogs runs synchronously
// in this same goroutine; its loop reads messages from that one connection.
func (s *tailSession) run(ctx context.Context) {
	defer close(s.events)
	reconnecting := false
	for ctx.Err() == nil {
		conn, err := s.repo.connect(ctx, &s.query)
		if err == nil {
			if reconnecting && !s.send(ctx, Event{Status: "Log connection restored."}) {
				_ = conn.Close()
				return
			}
			err = s.readLogs(ctx, conn)
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil && !isRetryable(err) {
			s.send(ctx, Event{Err: err})
			return
		}
		reconnecting = true
		if !s.send(ctx, Event{Status: fmt.Sprintf("Unable to connect to logs. Retrying in %s.", s.delay)}) {
			return
		}
		timer := time.NewTimer(s.delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		s.delay = min(2*s.delay, 30*time.Second)
	}
}

// readLogs delivers logs from one socket until cancellation, a read error,
// or a decode error. It closes the socket before returning.
func (s *tailSession) readLogs(ctx context.Context, conn *websocket.Conn) error {
	// Preserve the IDs from the start of this connection. For example, if we
	// already delivered A and B at time T, then receive C at T+1 followed by
	// B again, C clears s.boundaryIDs. This copy still lets us skip B.
	initialBoundaryIDs := maps.Clone(s.boundaryIDs)
	const pingInterval = 20 * time.Second
	const pongWait = 60 * time.Second
	// Pongs demonstrate that a quiet connection is responsive. Sending a ping
	// does not extend the deadline; the server must reply.
	readDeadline := time.Now().Add(pongWait)
	if err := conn.SetReadDeadline(readDeadline); err != nil {
		_ = conn.Close()
		return err
	}
	conn.SetPongHandler(func(string) error {
		// A reply demonstrates a responsive connection, even without logs.
		s.delay = time.Second
		readDeadline = time.Now().Add(pongWait)
		return conn.SetReadDeadline(readDeadline)
	})
	stopPings := make(chan struct{})
	pingsDone := make(chan struct{})
	go func() {
		defer close(pingsDone)
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopPings:
				return
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(pingInterval)); err != nil {
					// Unblock the reader so the reconnect loop can handle the failure.
					_ = conn.Close()
					return
				}
			}
		}
	}()
	defer func() {
		close(stopPings)
		_ = conn.Close()
		<-pingsDone
	}()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	for ctx.Err() == nil {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var entry lclient.Log
		if err := json.Unmarshal(message, &entry); err != nil {
			return fmt.Errorf("decode tailed log: %w", err)
		}
		if _, seen := initialBoundaryIDs[entry.Id]; seen {
			continue
		}
		if _, seen := s.boundaryIDs[entry.Id]; seen {
			continue
		}
		// A blocked consumer prevents us from reading pongs. Exclude that
		// wait from the heartbeat deadline before returning to socket reads.
		deliveryStarted := time.Now()
		if !s.send(ctx, Event{Log: &entry}) {
			return ctx.Err()
		}
		readDeadline = readDeadline.Add(time.Since(deliveryStarted))
		// Keep only IDs at the resume boundary, not the whole tail.
		// Logs arriving with timestamps earlier than latest are still delivered,
		// but do not move the resume timestamp backward.
		if entry.Timestamp.After(s.latest) {
			s.latest = entry.Timestamp
			clear(s.boundaryIDs)
			s.query.StartTime = &s.latest
		}
		if entry.Id != "" && !entry.Timestamp.IsZero() && entry.Timestamp.Equal(s.latest) {
			s.boundaryIDs[entry.Id] = struct{}{}
		}
		// Delivering a log also demonstrates a healthy connection.
		s.delay = time.Second
		if err := conn.SetReadDeadline(readDeadline); err != nil {
			return err
		}
	}
	return ctx.Err()
}

// send delivers an event to the caller's channel, waiting for a consumer or
// cancellation. It returns true if the event was sent, or false if cancellation
// interrupted the wait. If both are ready, select may still send the event.
func (s *tailSession) send(ctx context.Context, event Event) bool {
	select {
	case s.events <- event:
		return true
	case <-ctx.Done():
		return false
	}
}

// connect opens one authenticated log subscription using the supplied filters.
// Handshake failures retain the HTTP status for retry classification.
func (l *LogRepo) connect(ctx context.Context, params *client.ListLogsParams) (*websocket.Conn, error) {
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
			defer func() { _ = resp.Body.Close() }()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}

			return nil, &handshakeError{status: resp.StatusCode, err: fmt.Errorf("failed to tail logs (HTTP %d): %s", resp.StatusCode, body)}
		}

		return nil, err
	}

	return conn, nil
}

type handshakeError struct {
	status int
	err    error
}

func (e *handshakeError) Error() string { return e.err.Error() }
func (e *handshakeError) Unwrap() error { return e.err }

// isRetryable classifies failures for another subscription attempt. AsType matches
// wrapped error types (including net.Error); Is matches sentinel error values.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if handshake, ok := errors.AsType[*handshakeError](err); ok {
		return handshake.status == http.StatusTooManyRequests || handshake.status >= 500
	}
	if _, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		return false
	}
	if closeErr, ok := errors.AsType[*websocket.CloseError](err); ok {
		switch closeErr.Code {
		case websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure,
			websocket.CloseInternalServerErr, websocket.CloseServiceRestart, websocket.CloseTryAgainLater:
			return true
		default:
			return false
		}
	}
	_, networkError := errors.AsType[net.Error](err)
	return networkError || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

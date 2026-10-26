package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

// statusDoer answers each GET in-process with the next status, repeating the
// last one. synctest can't fake time across a real socket. With hang set, a
// GET blocks until its context ends.
type statusDoer struct {
	t        *testing.T
	statuses []sandboxclient.SandboxStatus
	calls    int
	hang     bool
}

func (d *statusDoer) Do(r *http.Request) (*http.Response, error) {
	require.Equal(d.t, http.MethodGet, r.Method)
	if d.hang {
		<-r.Context().Done()
		return nil, r.Context().Err()
	}
	status := d.statuses[min(d.calls, len(d.statuses)-1)]
	d.calls++
	body, err := json.Marshal(sandboxclient.Sandbox{Id: "sbx-1", Status: status})
	require.NoError(d.t, err)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    r,
	}, nil
}

func newStatusDoerService(t *testing.T, statuses ...sandboxclient.SandboxStatus) (*Service, *statusDoer) {
	t.Helper()
	doer := &statusDoer{t: t, statuses: statuses}
	c, err := client.NewClientWithResponses("http://sandbox.test", client.WithHTTPClient(doer))
	require.NoError(t, err)
	return NewService(NewRepo(c)), doer
}

func TestWaitUntilRunning(t *testing.T) {
	t.Setenv("RENDER_WORKSPACE", "tea-123")

	const (
		creating   = sandboxclient.SandboxStatusCreating
		resuming   = sandboxclient.SandboxStatusResuming
		running    = sandboxclient.SandboxStatusRunning
		errored    = sandboxclient.SandboxStatusErrored
		terminated = sandboxclient.SandboxStatusTerminated
		suspended  = sandboxclient.SandboxStatusSuspended
	)

	tests := []struct {
		name      string
		statuses  []sandboxclient.SandboxStatus
		wantErr   string
		wantCalls int
		wantSeen  []sandboxclient.SandboxStatus
	}{
		{name: "already running", statuses: []sandboxclient.SandboxStatus{running}, wantCalls: 1, wantSeen: []sandboxclient.SandboxStatus{running}},
		{name: "creating then running", statuses: []sandboxclient.SandboxStatus{creating, creating, running}, wantCalls: 3, wantSeen: []sandboxclient.SandboxStatus{creating, running}},
		{name: "resuming then running", statuses: []sandboxclient.SandboxStatus{resuming, running}, wantCalls: 2, wantSeen: []sandboxclient.SandboxStatus{resuming, running}},
		{name: "errored", statuses: []sandboxclient.SandboxStatus{creating, errored}, wantErr: "sandbox sbx-1 is errored, not running", wantCalls: 2, wantSeen: []sandboxclient.SandboxStatus{creating, errored}},
		{name: "terminated", statuses: []sandboxclient.SandboxStatus{terminated}, wantErr: "sandbox sbx-1 is terminated, not running", wantCalls: 1, wantSeen: []sandboxclient.SandboxStatus{terminated}},
		{name: "suspended", statuses: []sandboxclient.SandboxStatus{suspended}, wantErr: "sandbox sbx-1 is suspended, not running", wantCalls: 1, wantSeen: []sandboxclient.SandboxStatus{suspended}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				svc, doer := newStatusDoerService(t, tc.statuses...)

				var seen []sandboxclient.SandboxStatus
				sb, err := svc.WaitUntilRunning(t.Context(), "sbx-1", func(sb *sandboxclient.Sandbox) {
					seen = append(seen, sb.Status)
				})

				if tc.wantErr != "" {
					require.EqualError(t, err, tc.wantErr)
				} else {
					require.NoError(t, err)
				}
				require.NotNil(t, sb, "the last sandbox comes back even with an error")
				assert.Equal(t, tc.statuses[tc.wantCalls-1], sb.Status)
				assert.Equal(t, tc.wantCalls, doer.calls)
				assert.Equal(t, tc.wantSeen, seen, "onEvent gets each new status once")
			})
		})
	}

	t.Run("times out after WaitTimeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			svc, doer := newStatusDoerService(t, creating)

			start := time.Now()
			sb, err := svc.WaitUntilRunning(t.Context(), "sbx-1", nil)

			require.EqualError(t, err, "sandbox sbx-1 did not run within 60s (last status: creating)")
			require.NotNil(t, sb)
			assert.Equal(t, WaitTimeout, time.Since(start))
			// Checks at 0, 0.25, 0.75, 1.75, and 3.75s, then every 2s.
			assert.Equal(t, 33, doer.calls)
		})
	})

	t.Run("the timeout covers a hung request", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			svc, doer := newStatusDoerService(t, creating)
			doer.hang = true

			start := time.Now()
			sb, err := svc.WaitUntilRunning(t.Context(), "sbx-1", nil)

			require.EqualError(t, err, "sandbox sbx-1 did not run within 60s")
			assert.Nil(t, sb)
			assert.Equal(t, WaitTimeout, time.Since(start))
		})
	})

	t.Run("stops when the context ends", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			svc, _ := newStatusDoerService(t, creating)
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			sb, err := svc.WaitUntilRunning(ctx, "sbx-1", nil)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.NotNil(t, sb)
		})
	})
}

package sandboxsnapshot_test

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
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/sandboxgroup"
	"github.com/render-oss/cli/pkg/sandboxsnapshot"
)

// snapshotDoer answers each GET in-process with the next snapshot, repeating
// the last one. synctest can't fake time across a real socket. With hang set,
// a GET blocks until its context ends.
type snapshotDoer struct {
	t         *testing.T
	snapshots []sandboxesclient.SandboxSnapshot
	paths     []string
	hang      bool
}

func (d *snapshotDoer) Do(r *http.Request) (*http.Response, error) {
	require.Equal(d.t, http.MethodGet, r.Method)
	if d.hang {
		<-r.Context().Done()
		return nil, r.Context().Err()
	}
	snapshot := d.snapshots[min(len(d.paths), len(d.snapshots)-1)]
	d.paths = append(d.paths, r.URL.Path)
	body, err := json.Marshal(snapshot)
	require.NoError(d.t, err)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    r,
	}, nil
}

func newSnapshotDoerService(t *testing.T, snapshots ...sandboxesclient.SandboxSnapshot) (*sandboxsnapshot.Service, *snapshotDoer) {
	t.Helper()
	doer := &snapshotDoer{t: t, snapshots: snapshots}
	c, err := client.NewClientWithResponses("http://snapshot.test", client.WithHTTPClient(doer))
	require.NoError(t, err)
	return sandboxsnapshot.NewService(sandboxsnapshot.NewRepo(c), sandboxgroup.NewService(sandboxgroup.NewRepo(c))), doer
}

func TestWaitUntilAvailable(t *testing.T) {
	t.Setenv("RENDER_WORKSPACE", testWorkspace)

	creating := sandboxesclient.SandboxSnapshot{Id: "snp-1", Status: sandboxesclient.SandboxSnapshotStatusCreating}
	available := sandboxesclient.SandboxSnapshot{Id: "snp-1", Status: sandboxesclient.SandboxSnapshotStatusAvailable}

	t.Run("polls until available", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			svc, doer := newSnapshotDoerService(t, creating, creating, available)

			snapshot, err := svc.WaitUntilAvailable(t.Context(), "sbg-1", "snp-1")
			require.NoError(t, err)
			assert.Equal(t, sandboxesclient.SandboxSnapshotStatusAvailable, snapshot.Status)
			assert.Len(t, doer.paths, 3)
			assert.Equal(t, "/sandbox-groups/sbg-1/snapshots/snp-1", doer.paths[0])
		})
	})

	t.Run("fails with the capture error", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			reason := "capture timed out"
			svc, _ := newSnapshotDoerService(t, sandboxesclient.SandboxSnapshot{
				Id:     "snp-1",
				Status: sandboxesclient.SandboxSnapshotStatusFailed,
				Error:  &reason,
			})

			snapshot, err := svc.WaitUntilAvailable(t.Context(), "sbg-1", "snp-1")
			require.EqualError(t, err, "snapshot snp-1 failed to capture: capture timed out")
			require.NotNil(t, snapshot, "the last snapshot comes back even with an error")
		})
	})

	t.Run("times out after WaitTimeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			svc, doer := newSnapshotDoerService(t, creating)

			start := time.Now()
			snapshot, err := svc.WaitUntilAvailable(t.Context(), "sbg-1", "snp-1")

			require.EqualError(t, err, "snapshot snp-1 was not available within 5m (last status: creating)")
			require.NotNil(t, snapshot)
			assert.Equal(t, sandboxsnapshot.WaitTimeout, time.Since(start))
			// Checks at 0, 0.25, 0.75, 1.75, and 3.75s, then every 2s.
			assert.Len(t, doer.paths, 153)
		})
	})

	t.Run("the timeout covers a hung request", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			svc, doer := newSnapshotDoerService(t, creating)
			doer.hang = true

			start := time.Now()
			_, err := svc.WaitUntilAvailable(t.Context(), "sbg-1", "snp-1")

			require.EqualError(t, err, "snapshot snp-1 was not available within 5m")
			assert.Equal(t, sandboxsnapshot.WaitTimeout, time.Since(start))
		})
	})

	t.Run("stops when the context ends", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			svc, _ := newSnapshotDoerService(t, creating)
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			snapshot, err := svc.WaitUntilAvailable(ctx, "sbg-1", "snp-1")
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.NotNil(t, snapshot)
		})
	})
}

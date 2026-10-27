package sandboximage_test

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
	"github.com/render-oss/cli/pkg/sandboximage"
)

// imageDoer answers each GET in-process with the next image, repeating the
// last one. synctest can't fake time across a real socket. With hang set, a
// GET blocks until its context ends.
type imageDoer struct {
	t      *testing.T
	images []sandboxesclient.SandboxBuild
	paths  []string
	hang   bool
}

func (d *imageDoer) Do(r *http.Request) (*http.Response, error) {
	require.Equal(d.t, http.MethodGet, r.Method)
	if d.hang {
		<-r.Context().Done()
		return nil, r.Context().Err()
	}
	image := d.images[min(len(d.paths), len(d.images)-1)]
	d.paths = append(d.paths, r.URL.Path)
	body, err := json.Marshal(image)
	require.NoError(d.t, err)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    r,
	}, nil
}

func newImageDoerService(t *testing.T, images ...sandboxesclient.SandboxBuild) (*sandboximage.Service, *imageDoer) {
	t.Helper()
	doer := &imageDoer{t: t, images: images}
	c, err := client.NewClientWithResponses("http://image.test", client.WithHTTPClient(doer))
	require.NoError(t, err)
	return sandboximage.NewService(sandboximage.NewRepo(c), sandboxgroup.NewService(sandboxgroup.NewRepo(c))), doer
}

func TestWaitUntilReady(t *testing.T) {
	t.Setenv("RENDER_WORKSPACE", testWorkspace)

	running := sandboxesclient.SandboxBuild{Id: "sbb-1", Status: sandboxesclient.SandboxBuildStatusRunning}
	succeeded := sandboxesclient.SandboxBuild{Id: "sbb-1", Status: sandboxesclient.SandboxBuildStatusSucceeded}

	t.Run("polls until succeeded", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			svc, doer := newImageDoerService(t, running, running, succeeded)

			image, err := svc.WaitUntilReady(t.Context(), "sbg-1", "sbb-1")
			require.NoError(t, err)
			assert.Equal(t, sandboxesclient.SandboxBuildStatusSucceeded, image.Status)
			assert.Len(t, doer.paths, 3)
			assert.Equal(t, "/sandbox-groups/sbg-1/builds/sbb-1", doer.paths[0])
		})
	})

	t.Run("fails with the failure reason", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			reason := "image is not linux/amd64"
			svc, _ := newImageDoerService(t, sandboxesclient.SandboxBuild{
				Id:            "sbb-1",
				Status:        sandboxesclient.SandboxBuildStatusFailed,
				FailureReason: &reason,
			})

			image, err := svc.WaitUntilReady(t.Context(), "sbg-1", "sbb-1")
			require.EqualError(t, err, "image sbb-1 failed to import: image is not linux/amd64")
			require.NotNil(t, image, "the last image comes back even with an error")
		})
	})

	t.Run("times out after WaitTimeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			svc, doer := newImageDoerService(t, running)

			start := time.Now()
			image, err := svc.WaitUntilReady(t.Context(), "sbg-1", "sbb-1")

			require.EqualError(t, err, "image sbb-1 was not ready within 10m (last status: running)")
			require.NotNil(t, image)
			assert.Equal(t, sandboximage.WaitTimeout, time.Since(start))
			// Checks at 0, 0.25, 0.75, 1.75, and 3.75s, then every 2s.
			assert.Len(t, doer.paths, 303)
		})
	})

	t.Run("the timeout covers a hung request", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			svc, doer := newImageDoerService(t, running)
			doer.hang = true

			start := time.Now()
			_, err := svc.WaitUntilReady(t.Context(), "sbg-1", "sbb-1")

			require.EqualError(t, err, "image sbb-1 was not ready within 10m")
			assert.Equal(t, sandboximage.WaitTimeout, time.Since(start))
		})
	})

	t.Run("stops when the context ends", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			svc, _ := newImageDoerService(t, running)
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			image, err := svc.WaitUntilReady(ctx, "sbg-1", "sbb-1")
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.NotNil(t, image)
		})
	})
}

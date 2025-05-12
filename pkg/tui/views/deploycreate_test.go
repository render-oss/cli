package views_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/tui/views"
)

func TestWaitForDeploy(t *testing.T) {
	getDeployCallCount := 0
	setupTestServer(t, func() (int, string) {
		getDeployCallCount++
		if getDeployCallCount == 1 {
			return http.StatusOK, fmt.Sprintf(deployRespTmpl, client.DeployStatusBuildInProgress)
		}

		return http.StatusOK, fmt.Sprintf(deployRespTmpl, client.DeployStatusLive)
	})

	dep, err := views.WaitForDeploy(context.Background(), "some-service-id", "some-deploy-id")
	require.NoError(t, err)

	assert.Equal(t, client.DeployStatusLive, *dep.Status)
	assert.Equal(t, "some-deploy-id", dep.Id)
}

func TestWaitForDeployCreate(t *testing.T) {
	listDeployCallCount := 0
	setupTestServer(t, func() (int, string) {
		listDeployCallCount++
		if listDeployCallCount == 1 {
			return http.StatusOK, `[]`
		}

		deploy := fmt.Sprintf(deployRespTmpl, client.DeployStatusQueued)
		return http.StatusOK, fmt.Sprintf(`[{"deploy": %s, "cursor": "some-cursor"}]`, deploy)
	})

	dep, err := views.WaitForDeployCreate(context.Background(), "some-service-id")
	require.NoError(t, err)

	assert.Equal(t, client.DeployStatusQueued, *dep.Status)
	assert.Equal(t, "some-deploy-id", dep.Id)
}

func setupTestServer(t *testing.T, handler func() (int, string)) *httptest.Server {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")

		code, resp := handler()
		w.WriteHeader(code)
		_, err := w.Write([]byte(resp))
		require.NoError(t, err)
	}))

	require.NoError(t, os.Setenv("RENDER_API_KEY", "test-key"))
	require.NoError(t, os.Setenv("RENDER_HOST", s.URL))

	return s
}

const deployRespTmpl = `{
  "commit": {
    "createdAt": "2022-09-23T15:34:12Z",
    "id": "a21fb02cd25b7be602c5becf7fcbe6cdb9764db8",
    "message": "Merge pull request #3"
  },
  "createdAt": "2024-12-03T17:02:30.548731Z",
  "finishedAt": "2024-12-03T17:04:01.515412Z",
  "id": "some-deploy-id",
  "status": "%s",
  "trigger": "api",
  "updatedAt": "2024-12-03T17:04:01.516462Z"
}`

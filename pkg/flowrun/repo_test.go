package flowrun

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	workflows "github.com/render-oss/cli/pkg/client/workflows"
)

func TestStartFlowRunNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	c, err := client.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	_, err = NewRepo(c).StartFlowRun(context.Background(), workflows.StartFlowRun{Flow: "My Workflow/my-flow"})
	require.EqualError(t, err, `flow "My Workflow/my-flow" not found (received response code 404: not found)`)
	var notFound *NotFoundError
	require.ErrorAs(t, err, &notFound)
	require.Equal(t, "My Workflow/my-flow", notFound.Flow)
	require.EqualError(t, errors.Unwrap(err), "received response code 404: not found")
}

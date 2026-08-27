package client_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/config"
)

// roundTripFunc adapts a function to the [http.RoundTripper] interface so this
// test can inspect the refresh request without waiting for a real timeout.
type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip calls f with req.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNewDefaultClient_OAuthRefreshTimeoutKeepsCurrentCredentials(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_PATH", "")
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("RENDER_API_KEY", "")
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: "tea-current-token"}))

	startConfig := config.APIConfig{
		Key:          "current-access-token",
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		Host:         server.URL(),
		RefreshToken: "retryable-refresh-token",
	}
	require.NoError(t, config.SetAPIConfig(startConfig))

	// oauth.NewClient uses http.DefaultClient, so replacing it intercepts the
	// refresh request made during client.NewDefaultClient. The Render API client
	// returned by NewDefaultClient is constructed with a separate *http.Client.
	refreshRequests := 0
	originalHTTPClient := http.DefaultClient
	t.Cleanup(func() {
		http.DefaultClient = originalHTTPClient
	})

	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		refreshRequests++
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, server.URL()+"/token/refresh/", req.URL.String())
		// Immediately return a DeadlineExceeded error to simulate the 5 second HTTP timeout
		deadline, ok := req.Context().Deadline()
		require.True(t, ok)
		require.WithinDuration(t, time.Now().Add(5*time.Second), deadline, time.Second)
		return nil, context.DeadlineExceeded
	})}

	// Constructing the default client automatically attemps to refresh stored OAuth
	// credentials that expire within 24 hours.
	gotClient, err := client.NewDefaultClient()
	require.NoError(t, err, "Refresh timeout should not result in an error")
	require.NotNil(t, gotClient)
	require.Equal(t, 1, refreshRequests, "expected exactly one OAuth refresh request")

	// Because gotClient does not use the overridden http.DefaultClient, this
	// request reaches the fake Render API rather than the timeout transport.
	owners, err := gotClient.ListOwnersWithResponse(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, *owners.JSON200, 1)

	gotConfig, err := config.OAuthConfig()
	require.NoError(t, err)
	require.Equal(t, startConfig, gotConfig, "config should be unmodified")
}

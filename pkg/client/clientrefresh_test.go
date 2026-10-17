package client_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
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

// setupStoredOAuthConfig isolates the CLI config and stores OAuth credentials
// whose access token expires within the 24 hour refresh window.
func setupStoredOAuthConfig(t *testing.T, host string) config.APIConfig {
	t.Helper()
	t.Setenv("RENDER_CLI_CONFIG_PATH", "")
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("RENDER_API_KEY", "")

	startConfig := config.APIConfig{
		Key:          "current-access-token",
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		Host:         host,
		RefreshToken: "stored-refresh-token",
	}
	require.NoError(t, config.SetAPIConfig(startConfig))
	return startConfig
}

func TestNewDefaultClient_OAuthRefreshFailureHandling(t *testing.T) {
	tests := []struct {
		name             string
		status           int
		header           map[string]string
		body             string
		wantRefreshToken bool
	}{
		{
			name:             "server unavailable preserves refresh token",
			status:           http.StatusServiceUnavailable,
			body:             `{"error":"temporarily_unavailable"}`,
			wantRefreshToken: true,
		},
		{
			name:             "rate limited preserves refresh token",
			status:           http.StatusTooManyRequests,
			header:           map[string]string{"Retry-After": "60"},
			body:             `{"error":"slow_down"}`,
			wantRefreshToken: true,
		},
		{
			name:             "internal server error without body preserves refresh token",
			status:           http.StatusInternalServerError,
			wantRefreshToken: true,
		},
		{
			name:             "malformed success response preserves refresh token",
			status:           http.StatusOK,
			body:             `not json`,
			wantRefreshToken: true,
		},
		{
			name:             "bad request without invalid_grant preserves refresh token",
			status:           http.StatusBadRequest,
			body:             `{"error":"invalid_request"}`,
			wantRefreshToken: true,
		},
		{
			name:             "invalid grant clears refresh token",
			status:           http.StatusBadRequest,
			body:             `{"error":"invalid_grant"}`,
			wantRefreshToken: false,
		},
		{
			name:             "unauthorized invalid grant clears refresh token",
			status:           http.StatusUnauthorized,
			body:             `{"error":"invalid_grant"}`,
			wantRefreshToken: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refreshRequests := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				refreshRequests++
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/token/refresh/", r.URL.Path)
				for k, v := range tt.header {
					w.Header().Set(k, v)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, err := w.Write([]byte(tt.body))
				require.NoError(t, err)
			}))
			t.Cleanup(s.Close)

			startConfig := setupStoredOAuthConfig(t, s.URL)

			gotClient, err := client.NewDefaultClient()
			require.NoError(t, err)
			require.NotNil(t, gotClient)
			require.Equal(t, 1, refreshRequests, "expected exactly one OAuth refresh request")

			gotConfig, err := config.OAuthConfig()
			require.NoError(t, err)
			wantConfig := startConfig
			if !tt.wantRefreshToken {
				wantConfig.RefreshToken = ""
			}
			require.Equal(t, wantConfig, gotConfig)
		})
	}
}

func TestNewDefaultClient_OAuthRefreshTransportErrorKeepsCurrentCredentials(t *testing.T) {
	startConfig := setupStoredOAuthConfig(t, "https://oauth.example.invalid")

	refreshRequests := 0
	originalHTTPClient := http.DefaultClient
	t.Cleanup(func() {
		http.DefaultClient = originalHTTPClient
	})
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		refreshRequests++
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
	})}

	gotClient, err := client.NewDefaultClient()
	require.NoError(t, err)
	require.NotNil(t, gotClient)
	require.Equal(t, 1, refreshRequests, "expected exactly one OAuth refresh request")

	gotConfig, err := config.OAuthConfig()
	require.NoError(t, err)
	require.Equal(t, startConfig, gotConfig, "config should be unmodified")
}

func TestNewDefaultClient_OAuthRefreshRecoversAfterTransientFailure(t *testing.T) {
	healthy := false
	refreshRequests := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshRequests++
		require.Equal(t, "/token/refresh/", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if !healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"temporarily_unavailable"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"rotated-access-token","token_type":"Bearer","expires_in":172800,"refresh_token":"rotated-refresh-token"}`))
	}))
	t.Cleanup(s.Close)

	startConfig := setupStoredOAuthConfig(t, s.URL)

	_, err := client.NewDefaultClient()
	require.NoError(t, err)
	gotConfig, err := config.OAuthConfig()
	require.NoError(t, err)
	require.Equal(t, startConfig, gotConfig, "transient failure should not modify config")

	healthy = true
	_, err = client.NewDefaultClient()
	require.NoError(t, err)
	require.Equal(t, 2, refreshRequests, "expected the next client to retry the refresh")

	gotConfig, err = config.OAuthConfig()
	require.NoError(t, err)
	require.Equal(t, "rotated-access-token", gotConfig.Key)
	require.Equal(t, "rotated-refresh-token", gotConfig.RefreshToken)
	require.Greater(t, gotConfig.ExpiresAt, startConfig.ExpiresAt)
}

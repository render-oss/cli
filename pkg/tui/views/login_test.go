package views

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/client/oauth"
	"github.com/render-oss/cli/pkg/config"
)

func TestIsAlreadyLoggedInDoesNotRequireWorkspace(t *testing.T) {
	t.Setenv("RENDER_API_KEY", "")
	t.Setenv("RENDER_HOST", "")
	t.Setenv("RENDER_WORKSPACE", "")
	t.Setenv("RENDER_CLI_CONFIG_PATH", filepath.Join(t.TempDir(), "cli.yaml"))

	server := renderapi.NewServer(t)
	server.SetCurrentUser(renderapi.NewUser(client.User{}))

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_oauth_token",
		Host: server.URL(),
	}))

	assert.True(t, isAlreadyLoggedIn(context.Background()))
	assert.True(t, server.HasRequest(http.MethodGet, "/users"))
	assert.False(t, server.HasRequest(http.MethodGet, "/owners/"))
}

func TestIsAlreadyLoggedInReturnsFalseWhenCurrentUserIsNotSeeded(t *testing.T) {
	t.Setenv("RENDER_API_KEY", "")
	t.Setenv("RENDER_HOST", "")
	t.Setenv("RENDER_WORKSPACE", "")
	t.Setenv("RENDER_CLI_CONFIG_PATH", filepath.Join(t.TempDir(), "cli.yaml"))

	server := renderapi.NewServer(t)

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_oauth_token",
		Host: server.URL(),
	}))

	assert.False(t, isAlreadyLoggedIn(context.Background()))
	assert.True(t, server.HasRequest(http.MethodGet, "/users"))
}

func TestLoginSavesTokenWhetherOrNotTheBrowserOpens(t *testing.T) {
	tests := []struct {
		name        string
		openBrowser func(string) error
		wantHint    bool
	}{
		{
			name:        "browser opens",
			openBrowser: func(string) error { return nil },
			wantHint:    false,
		},
		{
			name: "browser fails to open",
			openBrowser: func(string) error {
				return errors.New(`exec: "xdg-open": executable file not found in $PATH`)
			},
			wantHint: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RENDER_API_KEY", "")
			t.Setenv("RENDER_HOST", "")
			t.Setenv("RENDER_WORKSPACE", "")
			t.Setenv("RENDER_CLI_CONFIG_PATH", filepath.Join(t.TempDir(), "cli.yaml"))

			server := renderapi.NewServer(t)

			var out bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetContext(context.Background())

			require.NoError(t, login(cmd, oauth.NewClient(server.URL()), tt.openBrowser))

			apiCfg, err := config.OAuthConfig()
			require.NoError(t, err)
			assert.Equal(t, renderapi.OAuthAccessToken, apiCfg.Key)

			const hint = "Could not open your browser automatically"
			if tt.wantHint {
				assert.Contains(t, out.String(), hint)
			} else {
				assert.NotContains(t, out.String(), hint)
			}
		})
	}
}

func TestStartLoginProceedsWhenBrowserOpenFails(t *testing.T) {
	t.Setenv("RENDER_API_KEY", "")
	t.Setenv("RENDER_HOST", "")
	t.Setenv("RENDER_WORKSPACE", "")
	t.Setenv("RENDER_CLI_CONFIG_PATH", filepath.Join(t.TempDir(), "cli.yaml"))

	server := renderapi.NewServer(t)

	dc := oauth.NewClient(server.URL())
	msg := startLogin(context.Background(), dc, func(string) error {
		return errors.New(`exec: "xdg-open": executable file not found in $PATH`)
	})()

	started, ok := msg.(loginStartedMsg)
	require.True(t, ok, "expected loginStartedMsg, got %T: %v", msg, msg)
	assert.True(t, started.browserOpenFailed)
	assert.NotEmpty(t, started.dashURL)
}

func TestLoginViewShowsFallbackHintWhenBrowserOpenFails(t *testing.T) {
	l := &LoginView{dashURL: "https://dashboard.example.com/login", browserOpenFailed: true}

	assert.Contains(t, l.View(), "Could not open your browser automatically")
	assert.Contains(t, l.View(), "https://dashboard.example.com/login")
}

func TestPollForTokenContinuesWhileAuthorizationPending(t *testing.T) {
	requests := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/device-token", r.URL.Path)
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"some device token"}`))
	}))
	defer s.Close()

	token, err := pollForToken(context.Background(), oauth.NewClient(s.URL), &oauth.DeviceGrant{
		DeviceCode: "some device code",
		ExpiresIn:  10,
		Interval:   1,
	})
	require.NoError(t, err)
	assert.Equal(t, "some device token", token.AccessToken)
	assert.Equal(t, 2, requests)
}

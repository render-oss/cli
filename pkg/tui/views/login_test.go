package views

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
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

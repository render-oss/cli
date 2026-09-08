package views_test

import (
	"context"
	"testing"
	"time"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	logclient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/logs"
	"github.com/render-oss/cli/pkg/tui/views"
	"github.com/stretchr/testify/require"
)

func TestLogLoaderToParam(t *testing.T) {
	clearAuthEnv := func(t *testing.T) {
		t.Helper()
		t.Setenv("RENDER_WORKSPACE", "")
		t.Setenv("RENDER_API_KEY", "")
		t.Setenv("RENDER_CLI_CONFIG_PATH", t.TempDir()+"/nonexistent.yaml")
	}

	resourceID := "srv-abcdef1234567890abcd"

	t.Run("local does not require workspace", func(t *testing.T) {
		clearAuthEnv(t)

		loader := views.NewLocalLogLoader(nil)
		params, err := loader.ToParam(context.Background(), views.LogInput{
			ResourceIDs: []string{"wfl-local"},
		})
		require.NoError(t, err)
		require.Equal(t, "", params.OwnerId)
		require.Equal(t, []string{"wfl-local"}, params.Resource)
	})

	t.Run("production requires workspace", func(t *testing.T) {
		clearAuthEnv(t)

		loader := views.NewLogLoader(nil, nil, nil, nil, nil)
		_, err := loader.ToParam(context.Background(), views.LogInput{
			ResourceIDs: []string{resourceID},
		})
		require.ErrorContains(t, err, config.ErrNoWorkspace.Error())
	})

	t.Run("production includes workspace when set", func(t *testing.T) {
		clearAuthEnv(t)
		t.Setenv("RENDER_WORKSPACE", "wrk-test123")

		loader := views.NewLogLoader(nil, nil, nil, nil, nil)
		params, err := loader.ToParam(context.Background(), views.LogInput{
			ResourceIDs: []string{resourceID},
		})
		require.NoError(t, err)
		require.Equal(t, "wrk-test123", params.OwnerId)
		require.Equal(t, []string{resourceID}, params.Resource)
	})
}

func TestLoadLogStreamUsesExistingRepositoryAndClosesOnCancel(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Logs.QueueStream(renderapi.LogStreamAttempt{
		Logs: []logclient.Log{{
			Id: "log-1", Message: "hello", Timestamp: time.Unix(100, 0).UTC(),
		}},
		HoldOpen: true,
	})
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	loader := views.NewLocalLogLoader(logs.NewLogRepo(c, &config.APIConfig{Host: server.URL()}))
	ctx, cancel := context.WithCancel(context.Background())

	stream, err := loader.LoadLogStream(ctx, views.LogInput{
		ResourceIDs: []string{"job-fake0000000000000000"},
		Tail:        true,
	})
	require.NoError(t, err)
	<-server.Logs.Opened()
	entry := <-stream.Logs
	require.Equal(t, "hello", entry.Message)
	cancel()
	<-server.Logs.Closed()
	for range stream.Logs {
	}
	for range stream.Errors {
	}
}

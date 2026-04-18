package views_test

import (
	"context"
	"testing"

	"github.com/render-oss/cli/v2/pkg/config"
	"github.com/render-oss/cli/v2/pkg/tui/views"
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

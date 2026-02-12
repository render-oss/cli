package views_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
)

func setupTestConfig(t *testing.T, content string) {
	t.Helper()
	f, err := os.CreateTemp("", "render-config")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	t.Setenv("RENDER_CLI_CONFIG_PATH", f.Name())
}

func TestWorkspaceViewHighlightsCurrentWorkspace(t *testing.T) {
	owners := []*client.Owner{
		{Id: "tea-111", Name: "Team1", Email: "team1@test.com", Type: "team"},
		{Id: "tea-222", Name: "Team2", Email: "team2@test.com", Type: "team"},
		{Id: "tea-333", Name: "Team3", Email: "team3@test.com", Type: "team"},
	}

	t.Run("highlights the current workspace", func(t *testing.T) {
		setupTestConfig(t, "version: 1\nworkspace: tea-222\nworkspace_name: Team2\n")

		view := views.NewWorkspaceView(context.Background(), views.ListWorkspaceInput{})
		model, _ := view.Update(tui.LoadDataMsg[[]*client.Owner]{Data: owners})

		table, ok := model.(*tui.Table[*client.Owner])
		require.True(t, ok)
		assert.Equal(t, 1, table.Model.GetHighlightedRowIndex())
	})

	t.Run("highlights first row when no workspace is set", func(t *testing.T) {
		setupTestConfig(t, "version: 1\n")

		view := views.NewWorkspaceView(context.Background(), views.ListWorkspaceInput{})
		model, _ := view.Update(tui.LoadDataMsg[[]*client.Owner]{Data: owners})

		table, ok := model.(*tui.Table[*client.Owner])
		require.True(t, ok)
		assert.Equal(t, 0, table.Model.GetHighlightedRowIndex())
	})

	t.Run("highlights first row when workspace ID has no match", func(t *testing.T) {
		setupTestConfig(t, "version: 1\nworkspace: tea-999\nworkspace_name: Missing\n")

		view := views.NewWorkspaceView(context.Background(), views.ListWorkspaceInput{})
		model, _ := view.Update(tui.LoadDataMsg[[]*client.Owner]{Data: owners})

		table, ok := model.(*tui.Table[*client.Owner])
		require.True(t, ok)
		assert.Equal(t, 0, table.Model.GetHighlightedRowIndex())
	})
}

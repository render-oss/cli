package text_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/text"
)

func TestWorkspaceTable(t *testing.T) {
	t.Run("formats workspaces correctly", func(t *testing.T) {
		workspaces := []*client.Owner{
			{
				Name:  "My Workspace",
				Email: "user@example.com",
				Id:    "tea-abc123",
			},
			{
				Name:  "Team Workspace",
				Email: "team@example.com",
				Id:    "tea-def456",
			},
		}

		result := text.WorkspaceTable(workspaces)

		assert.Contains(t, result, "NAME")
		assert.Contains(t, result, "EMAIL")
		assert.Contains(t, result, "ID")
		assert.Contains(t, result, "My Workspace")
		assert.Contains(t, result, "user@example.com")
		assert.Contains(t, result, "tea-abc123")
		assert.Contains(t, result, "Team Workspace")
		assert.Contains(t, result, "team@example.com")
		assert.Contains(t, result, "tea-def456")
	})

	t.Run("handles empty list", func(t *testing.T) {
		workspaces := []*client.Owner{}

		result := text.WorkspaceTable(workspaces)

		assert.Contains(t, result, "NAME")
		assert.Contains(t, result, "EMAIL")
		assert.Contains(t, result, "ID")
		// Should only have header, no data rows
		lines := strings.Split(strings.TrimSpace(result), "\n")
		assert.Equal(t, 1, len(lines))
	})

	t.Run("handles single workspace", func(t *testing.T) {
		workspaces := []*client.Owner{
			{
				Name:  "Solo Workspace",
				Email: "solo@example.com",
				Id:    "usr-solo123",
			},
		}

		result := text.WorkspaceTable(workspaces)

		assert.Contains(t, result, "Solo Workspace")
		assert.Contains(t, result, "solo@example.com")
		assert.Contains(t, result, "usr-solo123")
	})
}

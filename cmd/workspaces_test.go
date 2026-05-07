package cmd

import (
	"testing"

	"github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaces_NonInteractive_ListsWorkspaces(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Name: "acme-corp", Email: "admin@acme.com"}))
	server.Owners.Add(renderapi.NewOwner(client.Owner{Name: "side-project", Email: "me@example.com"}))

	result, err := executeCommand(t, server, "workspaces", "--output", "text")

	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "acme-corp")
	assert.Contains(t, result.Stdout, "side-project")
	assert.True(t, server.HasRequest("GET", "/owners"), "expected GET /owners to be called")
}

func TestWorkspaces_NonInteractive_EmptyList(t *testing.T) {
	server := renderapi.NewServer(t)

	result, err := executeCommand(t, server, "workspaces", "--output", "text")

	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "NAME", "expected header row to be present")
	assert.NotContains(t, result.Stdout, "@", "expected no workspace rows — all workspace emails contain @")
	assert.True(t, server.HasRequest("GET", "/owners"), "expected GET /owners to be called")
}

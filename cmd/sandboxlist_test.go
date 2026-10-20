package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/sandbox"
)

func TestSandboxList_StatusHelpListsEveryStatus(t *testing.T) {
	server := renderapi.NewServer(t)

	result, err := executeSandboxCommand(t, server, "ea", "sandboxes", "list", "--help")
	require.NoError(t, err)

	for _, status := range sandbox.AllStatuses() {
		assert.Contains(t, result.Stdout, string(status), "--status help should list %q", status)
	}
}

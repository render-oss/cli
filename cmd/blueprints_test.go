package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
)

func TestBlueprintHelp(t *testing.T) {
	t.Run("early access help lists blueprints", func(t *testing.T) {
		server := renderapi.NewServer(t)

		result, output := executeBlueprintCommand(t, server, []string{"ea", "--help"})

		require.Equal(t, 0, result.ExitCode)
		require.Regexp(t, `(?m)^[ \t]+blueprints[ \t]+`, output.Stdout)
		require.Empty(t, server.Requests)
	})

	t.Run("blueprint help lists lifecycle commands", func(t *testing.T) {
		server := renderapi.NewServer(t)

		result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "--help"})

		require.Equal(t, 0, result.ExitCode)
		for _, name := range []string{"create", "update", "plan", "approve"} {
			require.Regexp(t, `(?m)^[ \t]+`+name+`[ \t]+`, output.Stdout)
		}
		require.Empty(t, server.Requests)
	})
}

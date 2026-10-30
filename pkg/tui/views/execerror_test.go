package views

import (
	"errors"
	"testing"

	"github.com/render-oss/cli/pkg/tui"
	"github.com/stretchr/testify/require"
)

func TestDatastoreExecErrorsShowUnderlyingError(t *testing.T) {
	childErr := errors.New("exit status 1")

	for name, wrapped := range map[string]error{
		"kv-cli": handleRedisCLIError(childErr),
		"psql":   handlePSQLError(PSQL)(childErr),
		"pgcli":  handlePSQLError(PGCLI)(childErr),
	} {
		t.Run(name, func(t *testing.T) {
			m := tui.NewErrorModel(wrapped)
			m.Update(tui.StackSizeMsg{Width: 120, Height: 20})
			view := m.View()
			require.Contains(t, view, "An error occurred while running "+name)
			require.Contains(t, view, "exit status 1")
		})
	}
}

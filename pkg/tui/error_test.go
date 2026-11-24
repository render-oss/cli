package tui_test

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/render-oss/cli/pkg/tui"
	"github.com/stretchr/testify/require"
)

func renderErrorView(err error) string {
	m := tui.NewErrorModel(err)
	m.Update(tui.StackSizeMsg{Width: 120, Height: 20})
	return m.View()
}

func TestErrorModelView(t *testing.T) {
	t.Run("wrapped user facing error without message shows underlying error", func(t *testing.T) {
		view := renderErrorView(tui.UserFacingError{
			Title: "An error occurred while running kv-cli",
			Err:   errors.New("exit status 1"),
		})
		require.Contains(t, view, "An error occurred while running kv-cli")
		require.Contains(t, view, "exit status 1")
	})

	t.Run("child process exit error is shown", func(t *testing.T) {
		falsePath, err := exec.LookPath("false")
		if err != nil {
			t.Skip("false not available")
		}
		runErr := exec.Command(falsePath).Run()
		var exitErr *exec.ExitError
		require.ErrorAs(t, runErr, &exitErr)

		view := renderErrorView(tui.UserFacingError{
			Title: "An error occurred while running psql",
			Err:   runErr,
		})
		require.Contains(t, view, "An error occurred while running psql")
		require.Contains(t, view, runErr.Error())
	})

	t.Run("user facing error nested in another error shows underlying error", func(t *testing.T) {
		view := renderErrorView(fmt.Errorf("context: %w", tui.UserFacingError{
			Title: "An error occurred while running pgcli",
			Err:   errors.New("exit status 2"),
		}))
		require.Contains(t, view, "An error occurred while running pgcli")
		require.Contains(t, view, "exit status 2")
	})

	t.Run("explicit message takes precedence over underlying error", func(t *testing.T) {
		view := renderErrorView(tui.UserFacingError{
			Title:   "Failed to SSH",
			Message: "explicit message",
			Err:     errors.New("hidden underlying error"),
		})
		require.Contains(t, view, "Failed to SSH")
		require.Contains(t, view, "explicit message")
		require.NotContains(t, view, "hidden underlying error")
	})

	t.Run("title only error shows title", func(t *testing.T) {
		view := renderErrorView(tui.UserFacingError{Title: "only a title"})
		require.Contains(t, view, "only a title")
		require.NotContains(t, view, "unknown error")
	})

	t.Run("message only error shows message", func(t *testing.T) {
		view := renderErrorView(tui.UserFacingError{Message: "No Key Value named 'x'."})
		require.Contains(t, view, "No Key Value named 'x'.")
	})

	t.Run("ordinary error shows error string", func(t *testing.T) {
		view := renderErrorView(errors.New("plain failure"))
		require.Contains(t, view, "plain failure")
	})
}

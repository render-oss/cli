package command

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewExitError(t *testing.T) {
	t.Run("without a cause", func(t *testing.T) {
		err := NewExitError(7, nil)

		require.EqualError(t, err, "exited with code 7")
		require.NoError(t, errors.Unwrap(err))

		var exitCoder ExitCoder
		require.ErrorAs(t, err, &exitCoder)
		require.Equal(t, 7, exitCoder.ExitCode())
	})

	t.Run("with a cause", func(t *testing.T) {
		cause := errors.New("remote command failed")
		err := NewExitError(7, cause)

		require.EqualError(t, err, cause.Error())
		require.ErrorIs(t, err, cause)
	})

	t.Run("with zero exit code", func(t *testing.T) {
		// Zero is misuse because callers should return nil for success, but constructing
		// the error must remain safe for runtime-provided codes. [cmd.Execute] defensively
		// maps a non-nil exit error with code zero to exit code 1.
		var err error
		require.NotPanics(t, func() {
			err = NewExitError(0, nil)
		})

		require.EqualError(t, err, "exited with code 0")
	})
}

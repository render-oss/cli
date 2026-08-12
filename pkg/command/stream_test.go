package command

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"
)

// red builds the string "text in red" using s's renderer and returns it
// without writing anything. Whether the returned string actually carries
// color escape codes is the thing under test: it depends on the color profile
// lipgloss resolved for s's stream. Tests use it as the cheapest style that
// disappears entirely when that profile is plain text.
func red(s *Stream, text string) string {
	return s.Renderer().NewStyle().Foreground(lipgloss.Color("9")).Render(text)
}

func clearColorEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE"} {
		t.Setenv(key, "")
	}
}

// Styling is delegated to lipgloss, which detects the color profile against
// the stream it is bound to. These cases pin the delegation: the CLI does not
// re-derive styling from its own TTY signals, so a redirected stream stays
// clean and a user who forces color still gets it.
func TestNewStream_StylingFollowsTheStream(t *testing.T) {
	clearColorEnvironment(t)

	testCases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "writer that is not a terminal gets no color",
			want: "warning",
		},
		{
			name: "NO_COLOR is honored",
			env:  map[string]string{"NO_COLOR": "1"},
			want: "warning",
		},
		{
			name: "CLICOLOR_FORCE is honored even though the stream is not a terminal",
			env:  map[string]string{"CLICOLOR_FORCE": "1"},
			want: "\x1b[91mwarning\x1b[0m",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			var buf bytes.Buffer

			require.Equal(t, tc.want, red(NewStream(&buf), "warning"))
		})
	}
}

func TestStream_StyledOutputToARedirectedFileIsPlain(t *testing.T) {
	clearColorEnvironment(t)

	file, err := os.Create(filepath.Join(t.TempDir(), "redirected.log"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })
	s := NewStream(file)

	_, err = fmt.Fprintln(s, red(s, "warning"))
	require.NoError(t, err)
	output, err := os.ReadFile(file.Name())

	require.NoError(t, err)
	require.Equal(t, "warning\n", string(output))
}

func TestStream_Write_PassesThroughToUnderlyingWriter(t *testing.T) {
	var buf bytes.Buffer
	s := NewStream(&buf)

	n, err := fmt.Fprintln(s, "hello", "world")

	require.NoError(t, err)
	require.Equal(t, len("hello world\n"), n)
	require.Equal(t, "hello world\n", buf.String())
}

// Width reports 0 for everything that isn't a terminal. A real width needs a
// real terminal, so it isn't covered here.
func TestStream_Width_IsZeroWhenNotATerminal(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "redirected.log"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })

	require.Equal(t, 0, NewStream(&bytes.Buffer{}).Width(), "writer that is not a file")
	require.Equal(t, 0, NewStream(file).Width(), "file that is not a terminal")
}

func TestStream_NilAndZeroValue_AreSafe(t *testing.T) {
	clearColorEnvironment(t)

	testCases := []struct {
		name string
		s    *Stream
	}{
		{name: "nil receiver", s: nil},
		{name: "zero value", s: &Stream{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotPanics(t, func() {
				n, err := fmt.Fprintln(tc.s, "discarded")

				require.NoError(t, err)
				require.Equal(t, len("discarded\n"), n)
				require.Equal(t, 0, tc.s.Width())
				require.Equal(t, "plain", red(tc.s, "plain"))
			})
		})
	}
}

func TestStderrContext_RoundTripAndFallback(t *testing.T) {
	t.Run("round trips the Stream that was set", func(t *testing.T) {
		var buf bytes.Buffer
		want := NewStream(&buf)

		ctx := SetStderrInContext(context.Background(), want)
		got := StderrFromContext(ctx)

		require.Same(t, want, got)
	})

	t.Run("missing value falls back to os.Stderr", func(t *testing.T) {
		s := StderrFromContext(context.Background())

		require.NotNil(t, s)
		require.NotNil(t, s.Renderer())
	})

	t.Run("nil Stream on the context falls back too", func(t *testing.T) {
		s := StderrFromContext(SetStderrInContext(context.Background(), nil))

		require.NotNil(t, s)
		require.NotNil(t, s.Renderer())
	})
}

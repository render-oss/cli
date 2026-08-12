package command

import (
	"context"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
)

// Stream is used to wrap one of the CLI's output streams: an io.Writer that carries the
// lipgloss renderer bound to it. Print to it with fmt.Fprintln/Fprintf and
// style with Renderer().
//
// Example:
//
//	stream := NewStream(os.Stderr)
//	warning := stream.Renderer().NewStyle().Bold(true).Render("warning")
//	fmt.Fprintln(stream, warning)
//
// The styling degrades to match wherever the bytes are actually going:
// color for a human at a terminal, plain text otherwise (e.g., a log file, a pipe, or an agent reading the output).
//
// Every style in pkg/style is bound to lipgloss's global renderer, which detects its color profile against stdout,
// so styling a second stream through those globals gets it wrong:
// [1] `render deploy 2>errors.log` writes raw ANSI into the log, and
// [2] `render services list | jq` strips color a human should still see.
// Binding a renderer to the stream being written to fixes both.
//
// A nil or zero-value Stream is safe: writes go to io.Discard.
type Stream struct {
	w        io.Writer
	renderer *lipgloss.Renderer
}

// NewStream builds a Stream for w.
func NewStream(w io.Writer) *Stream {
	return &Stream{
		w:        w,
		renderer: lipgloss.NewRenderer(w),
	}
}

// Write implements io.Writer, passing through to the underlying stream. A nil
// receiver or zero value discards rather than panicking, so callers never need
// to guard. The returned count and error are the underlying writer's own;
// best-effort callers ignore them at the call site as they would for any other
// stderr write.
func (s *Stream) Write(p []byte) (int, error) {
	return s.out().Write(p)
}

// out returns the underlying writer, discarding for a nil receiver or a zero
// value that was never built by NewStream.
func (s *Stream) out() io.Writer {
	if s == nil || s.w == nil {
		return io.Discard
	}
	return s.w
}

// Renderer returns the lipgloss renderer bound to this stream. A nil receiver
// or zero value returns a renderer over io.Discard.
func (s *Stream) Renderer() *lipgloss.Renderer {
	if s == nil || s.renderer == nil {
		return discardRenderer
	}
	return s.renderer
}

// Width reports how many columns wide this stream's terminal is.The width is measured on each call.
//
// Zero means "unknown", not "unlimited" (e.g., stream is redirected, piped, or not a file). A caller that forwards 0 straight to
// lipgloss gets no wrapping at all, since lipgloss only wraps for widths > 0.
func (s *Stream) Width() int {
	if s == nil {
		return 0
	}

	f, ok := s.w.(*os.File)
	if !ok {
		return 0
	}

	width, _, err := term.GetSize(f.Fd())
	if err != nil {
		return 0
	}
	return width
}

// discardRenderer backs Renderer() for a nil or zero-value Stream.
var discardRenderer = lipgloss.NewRenderer(io.Discard)

type ctxStderrKey struct{}

// SetStderrInContext carries the CLI's stderr Stream on ctx, mirroring
// SetFormatInContext. Stream itself is stream-neutral; these helpers are what
// make one of them the stderr stream.
func SetStderrInContext(ctx context.Context, s *Stream) context.Context {
	return context.WithValue(ctx, ctxStderrKey{}, s)
}

// StderrFromContext returns the stderr Stream carried on ctx. When none is set
// it returns one over os.Stderr, so call sites can use the result
// unconditionally.
func StderrFromContext(ctx context.Context) *Stream {
	if ctx != nil {
		if s, ok := ctx.Value(ctxStderrKey{}).(*Stream); ok && s != nil {
			return s
		}
	}
	return NewStream(os.Stderr)
}

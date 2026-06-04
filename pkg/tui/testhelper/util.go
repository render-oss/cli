package testhelper

import (
	"bytes"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/render-oss/cli/pkg/tui"
)

func Stackify(m tea.Model) tea.Model {
	stack := tui.NewStack()
	stack.Push(tui.ModelWithCmd{Model: m, Cmd: ""})
	return stack
}

type WaitForContainsOptions struct {
	Duration      time.Duration
	CheckInterval time.Duration
}

var defaultWaitForContainsOptions = WaitForContainsOptions{
	Duration:      3 * time.Second,
	CheckInterval: 10 * time.Millisecond,
}

// WaitForContains waits until output contains text, failing the test if the text
// does not appear before the timeout. Any zero-valued option fields are filled
// with defaults.
func WaitForContains(t testing.TB, output io.Reader, text string, opts ...WaitForContainsOptions) {
	t.Helper()
	options := waitForContainsOptions(opts...)
	teatest.WaitFor(t, output, func(b []byte) bool {
		return bytes.Contains(b, []byte(text))
	},
		teatest.WithCheckInterval(options.CheckInterval),
		teatest.WithDuration(options.Duration),
	)
}

func waitForContainsOptions(opts ...WaitForContainsOptions) WaitForContainsOptions {
	if len(opts) > 1 {
		panic("WaitForContains accepts at most one options struct")
	}
	options := defaultWaitForContainsOptions
	if len(opts) == 1 {
		options = mergeWaitForContainsOptions(options, opts[0])
	}
	return options
}

func mergeWaitForContainsOptions(base, override WaitForContainsOptions) WaitForContainsOptions {
	options := base
	if override.Duration != 0 {
		options.Duration = override.Duration
	}
	if override.CheckInterval != 0 {
		options.CheckInterval = override.CheckInterval
	}
	return options
}

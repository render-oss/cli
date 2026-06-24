package sandbox

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadSandboxExecStream(t *testing.T) {
	t.Run("output events and exit code", func(t *testing.T) {
		body := `event: output
data: {"stream":"stdout","data":"hello\n"}

event: output
data: {"stream":"stderr","data":"warn\n"}

event: exit
data: {"exit_code":7}
`
		var outputs []ExecOutputEvent
		exitCode, err := readSandboxExecStream(strings.NewReader(body), func(output *ExecOutputEvent) error {
			outputs = append(outputs, *output)
			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, 7, exitCode)
		require.Len(t, outputs, 2)
		assert.Equal(t, ExecOutputStreamStdout, outputs[0].Stream)
		assert.Equal(t, "hello\n", outputs[0].Data)
		assert.Equal(t, ExecOutputStreamStderr, outputs[1].Stream)
		assert.Equal(t, "warn\n", outputs[1].Data)
	})

	t.Run("terminal error event", func(t *testing.T) {
		body := `event: error
data: {"status":500,"message":"stream broke"}
`
		_, err := readSandboxExecStream(strings.NewReader(body), nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "stream broke")
	})

	t.Run("malformed output event", func(t *testing.T) {
		body := `event: output
data: {invalid json}
`
		_, err := readSandboxExecStream(strings.NewReader(body), nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing sandbox exec output")
	})

	t.Run("missing exit event", func(t *testing.T) {
		body := `event: output
data: {"stream":"stdout","data":"hello\n"}
`
		_, err := readSandboxExecStream(strings.NewReader(body), nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no sandbox exec exit event")
	})

	t.Run("output event larger than scanner buffer", func(t *testing.T) {
		// A single exec output event has no upper bound; one very long line
		// (here ~4MB) must not be truncated or rejected. This would fail under
		// a bufio.Scanner with a fixed max token size.
		payload := strings.Repeat("x", 4*1024*1024)
		body := `event: output
data: {"stream":"stdout","data":"` + payload + `"}

event: exit
data: {"exit_code":0}
`
		var outputs []ExecOutputEvent
		exitCode, err := readSandboxExecStream(strings.NewReader(body), func(output *ExecOutputEvent) error {
			outputs = append(outputs, *output)
			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		require.Len(t, outputs, 1)
		assert.Equal(t, payload, outputs[0].Data)
	})
}

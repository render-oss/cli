package sandbox

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	ExecOutputStreamStdout ExecOutputStream = "stdout"
	ExecOutputStreamStderr ExecOutputStream = "stderr"
)

type ExecOutputStream string

type ExecOutputEvent struct {
	Stream ExecOutputStream `json:"stream"`
	Data   string           `json:"data"`
}

type execExitEvent struct {
	ExitCode int `json:"exit_code"`
}

type execErrorEvent struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// readSandboxExecStream parses finite SSE events from an exec response. It
// invokes onOutput for each stdout/stderr chunk and returns the terminal process
// exit code from the "exit" event.
//
// We read with bufio.Reader rather than bufio.Scanner: a single exec output
// event has no upper bound (a command can emit one very long line — a minified
// file, base64 blob, or unflushed output), and bufio.Scanner fails with
// ErrTooLong once a token exceeds its max buffer. ReadString grows the buffer
// as needed, so there is no event-size ceiling to tune.
func readSandboxExecStream(r io.Reader, onOutput func(*ExecOutputEvent) error) (int, error) {
	reader := bufio.NewReader(r)

	var (
		event    string
		data     string
		exitCode *int
	)

	processEvent := func() error {
		if event == "" && data == "" {
			return nil
		}
		switch event {
		case "output":
			var output ExecOutputEvent
			if err := json.Unmarshal([]byte(data), &output); err != nil {
				return fmt.Errorf("parsing sandbox exec output from SSE data: %w", err)
			}
			if onOutput != nil {
				return onOutput(&output)
			}
			return nil
		case "exit":
			var exit execExitEvent
			if err := json.Unmarshal([]byte(data), &exit); err != nil {
				return fmt.Errorf("parsing sandbox exec exit from SSE data: %w", err)
			}
			exitCode = &exit.ExitCode
			return nil
		case "error":
			var streamErr execErrorEvent
			if err := json.Unmarshal([]byte(data), &streamErr); err != nil {
				return fmt.Errorf("parsing sandbox exec error from SSE data: %w", err)
			}
			return fmt.Errorf("sandbox exec stream error status %d: %s", streamErr.Status, streamErr.Message)
		default:
			return fmt.Errorf("unknown sandbox exec SSE event %q", event)
		}
	}

	handleLine := func(line string) error {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			if err := processEvent(); err != nil {
				return err
			}
			event = ""
			data = ""
			return nil
		}
		if after, ok := strings.CutPrefix(line, "event:"); ok {
			event = strings.TrimSpace(after)
			return nil
		}
		if after, ok := strings.CutPrefix(line, "data:"); ok {
			if data != "" {
				data += "\n"
			}
			data += strings.TrimPrefix(after, " ")
		}
		return nil
	}

	for {
		line, readErr := reader.ReadString('\n')
		line = strings.TrimSuffix(line, "\n")
		// On io.EOF, ReadString returns any final line that wasn't newline
		// terminated; process it before breaking.
		if line != "" || readErr == nil {
			if err := handleLine(line); err != nil {
				return 0, err
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				return 0, fmt.Errorf("reading sandbox exec SSE stream: %w", readErr)
			}
			break
		}
	}
	if err := processEvent(); err != nil {
		return 0, err
	}
	if exitCode == nil {
		return 0, fmt.Errorf("no sandbox exec exit event found in SSE response")
	}
	return *exitCode, nil
}

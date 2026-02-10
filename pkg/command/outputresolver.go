package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

type RuntimeSignals struct {
	StdinTTY     bool
	StdoutTTY    bool
	StderrTTY    bool
	DumbTerminal bool
	CI           bool
	ForcedOutput *Output
}

// SupportsInteractive returns true if the runtime environment can support
// an interactive TUI (all streams are TTYs, not a dumb terminal, not CI).
func (s RuntimeSignals) SupportsInteractive() bool {
	return s.StdinTTY && s.StdoutTTY && s.StderrTTY && !s.DumbTerminal && !s.CI
}

func DetectRuntimeSignals() (RuntimeSignals, error) {
	forcedOutput, err := detectForcedOutputFromEnv()
	if err != nil {
		return RuntimeSignals{}, err
	}

	return RuntimeSignals{
		StdinTTY:     isTTY(os.Stdin.Fd()),
		StdoutTTY:    isTTY(os.Stdout.Fd()),
		StderrTTY:    isTTY(os.Stderr.Fd()),
		DumbTerminal: os.Getenv("TERM") == "dumb",
		CI:           isTruthyEnv(os.Getenv("CI")),
		ForcedOutput: forcedOutput,
	}, nil
}

func isTTY(fd uintptr) bool {
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func ResolveAutoOutput(explicitOutputSet bool, requested Output, signals RuntimeSignals) (Output, error) {
	if !isSupportedOutput(requested) {
		return "", fmt.Errorf("invalid output format: %s", requested)
	}

	if explicitOutputSet {
		return requested, nil
	}

	if signals.ForcedOutput != nil {
		return *signals.ForcedOutput, nil
	}

	if signals.SupportsInteractive() {
		return Interactive, nil
	}

	return JSON, nil
}

func isSupportedOutput(output Output) bool {
	switch output {
	case Interactive, JSON, YAML, TEXT:
		return true
	default:
		return false
	}
}

func isTruthyEnv(value string) bool {
	return value == "1" || strings.EqualFold(value, "true")
}

func detectForcedOutputFromEnv() (*Output, error) {
	value := os.Getenv("RENDER_OUTPUT")
	if value == "" {
		return nil, nil
	}

	output, err := StringToOutput(value)
	if err != nil {
		return nil, fmt.Errorf("invalid RENDER_OUTPUT value: %s", value)
	}

	return &output, nil
}

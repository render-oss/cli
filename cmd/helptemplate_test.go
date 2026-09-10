package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestBoundedHelpTextWidth(t *testing.T) {
	tests := []struct {
		name          string
		terminalWidth int
		want          int
	}{
		{name: "unknown", terminalWidth: 0, want: maxHelpTextWidth},
		{name: "narrow terminal", terminalWidth: 48, want: 48},
		{name: "wide terminal", terminalWidth: 120, want: maxHelpTextWidth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, boundedHelpTextWidth(tt.terminalWidth))
		})
	}
}

func TestHelpTextWidthFallsBackForNonTerminalOutput(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})

	require.Equal(t, maxHelpTextWidth, helpTextWidth(cmd))
}

func TestWrapTextReflowsProseAndPreservesPreformattedLines(t *testing.T) {
	text := `First line with
manual break.

Examples:
  render example --long-flag`

	require.Equal(t, `First line with manual break.

Examples:
  render example --long-flag`, wrapText(text, 30))
}

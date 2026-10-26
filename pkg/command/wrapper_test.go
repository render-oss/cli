package command_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/command"
)

func TestNonInteractiveWithConfirm(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		confirm    bool
		wantLoad   bool
		wantPrompt bool
		wantAbort  bool
		wantEOF    bool
	}{
		{name: "closed stdin requires explicit confirmation", wantPrompt: true, wantEOF: true},
		{name: "unterminated input does not confirm", input: "y", wantPrompt: true, wantEOF: true},
		{name: "accepted prompt preserves JSON stdout", input: "y\n", wantLoad: true, wantPrompt: true},
		{name: "declined prompt preserves stdout", input: "n\n", wantPrompt: true, wantAbort: true},
		{name: "confirm flag skips stdin", confirm: true, wantLoad: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := command.JSON
			ctx := command.SetFormatInContext(context.Background(), &output)
			ctx = command.SetConfirmInContext(ctx, tt.confirm)
			cmd := &cobra.Command{}
			cmd.SetContext(ctx)
			cmd.SetIn(strings.NewReader(tt.input))
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)

			loaded := false
			handled, err := command.NonInteractiveWithConfirm(cmd, func() (string, error) {
				loaded = true
				return "created", nil
			}, func(data string) string { return data }, func() (string, error) {
				return "Proceed?", nil
			})

			if tt.wantEOF {
				require.ErrorIs(t, err, io.EOF)
				assert.Contains(t, err.Error(), "--confirm")
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantLoad, loaded)
			assert.Equal(t, tt.wantLoad, handled)
			if tt.wantLoad {
				assert.JSONEq(t, `"created"`, stdout.String())
			} else {
				assert.Empty(t, stdout.String())
			}
			if tt.wantPrompt {
				assert.Contains(t, stderr.String(), "Proceed? (y/n): ")
			} else {
				assert.Empty(t, stderr.String())
			}
			assert.Equal(t, tt.wantAbort, strings.Contains(stderr.String(), "Aborted\n"))
		})
	}
}

func TestNonInteractiveWithConfirmReadError(t *testing.T) {
	output := command.JSON
	cmd := &cobra.Command{}
	cmd.SetContext(command.SetFormatInContext(context.Background(), &output))
	readErr := errors.New("read failed")
	cmd.SetIn(iotest.ErrReader(readErr))
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	_, err := command.NonInteractiveWithConfirm(cmd, func() (string, error) {
		t.Fatal("action must not run when confirmation cannot be read")
		return "", nil
	}, func(data string) string { return data }, func() (string, error) { return "Proceed?", nil })

	require.ErrorIs(t, err, readErr)
	assert.Empty(t, stdout.String())
}

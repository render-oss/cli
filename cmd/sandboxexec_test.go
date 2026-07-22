package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/command"
)

func TestSandboxExecKeepsFileCompletion(t *testing.T) {
	requireCompletionDirective(t, []string{"ea", "sandboxes", "exec", ""}, cobra.ShellCompDirectiveDefault)
}

func TestExitSandboxExecUsesRemoteExitCode(t *testing.T) {
	cmd := &cobra.Command{Use: "exec"}
	err := exitSandboxExec(cmd, 7)
	var exitErr command.ExitCoder
	require.ErrorAs(t, err, &exitErr)
	require.Equal(t, 7, exitErr.ExitCode())
	require.True(t, cmd.SilenceErrors)
}

func TestExitSandboxExecSkipsZeroExitCode(t *testing.T) {
	err := exitSandboxExec(&cobra.Command{Use: "exec"}, 0)
	require.NoError(t, err)
}

func TestJoinShellCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "simple command", args: []string{"echo", "hello"}, want: "echo hello"},
		{name: "preserves quoted spaces", args: []string{"echo", "a b"}, want: "echo 'a b'"},
		{name: "safe punctuation unquoted", args: []string{"ls", "-la", "./path/to-file_1"}, want: "ls -la ./path/to-file_1"},
		{name: "quotes shell metacharacters", args: []string{"echo", "$HOME", "&&", "rm"}, want: "echo '$HOME' '&&' rm"},
		{name: "escapes embedded single quote", args: []string{"echo", "it's"}, want: `echo 'it'\''s'`},
		{name: "empty token", args: []string{"echo", ""}, want: "echo ''"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, joinShellCommand(tt.args))
		})
	}
}

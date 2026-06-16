package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
)

func TestWriteSandboxExecResultRawOutput(t *testing.T) {
	cmd, stdout, stderr := newSandboxExecTestCommand(command.TEXT)
	result := &sandboxclient.SandboxExecSyncResponse{
		Stdout:   "hello\n",
		Stderr:   "warning\n",
		ExitCode: 7,
	}

	require.NoError(t, writeSandboxExecResult(cmd, result))
	require.Equal(t, "hello\n", stdout.String())
	require.Equal(t, "warning\n", stderr.String())
}

func TestWriteSandboxExecResultJSONOutput(t *testing.T) {
	cmd, stdout, stderr := newSandboxExecTestCommand(command.JSON)
	result := &sandboxclient.SandboxExecSyncResponse{
		Stdout:   "hello\n",
		Stderr:   "warning\n",
		ExitCode: 7,
	}

	require.NoError(t, writeSandboxExecResult(cmd, result))
	require.JSONEq(t, `{"stdout":"hello\n","stderr":"warning\n","exitCode":7}`, stdout.String())
	require.Empty(t, stderr.String())
}

func TestWriteSandboxExecResultYAMLOutput(t *testing.T) {
	cmd, stdout, stderr := newSandboxExecTestCommand(command.YAML)
	result := &sandboxclient.SandboxExecSyncResponse{
		Stdout:   "hello\n",
		Stderr:   "warning\n",
		ExitCode: 7,
	}

	require.NoError(t, writeSandboxExecResult(cmd, result))

	var got map[string]any
	require.NoError(t, yaml.Unmarshal(stdout.Bytes(), &got))
	require.Equal(t, "hello\n", got["stdout"])
	require.Equal(t, "warning\n", got["stderr"])
	require.Equal(t, 7, got["exitCode"])
	require.Empty(t, stderr.String())
}

func TestExitSandboxExecUsesRemoteExitCode(t *testing.T) {
	oldExit := sandboxExecExit
	defer func() { sandboxExecExit = oldExit }()

	var gotExitCode *int
	sandboxExecExit = func(code int) {
		gotExitCode = &code
	}

	exitSandboxExec(7)
	require.NotNil(t, gotExitCode)
	require.Equal(t, 7, *gotExitCode)
}

func TestExitSandboxExecSkipsZeroExitCode(t *testing.T) {
	oldExit := sandboxExecExit
	defer func() { sandboxExecExit = oldExit }()

	called := false
	sandboxExecExit = func(int) {
		called = true
	}

	exitSandboxExec(0)
	require.False(t, called)
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

func newSandboxExecTestCommand(output command.Output) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	cmd := &cobra.Command{Use: "exec"}
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetContext(command.SetFormatInContext(context.Background(), &output))
	return cmd, stdout, stderr
}

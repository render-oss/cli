package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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

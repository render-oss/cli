package views

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPSQLResult_JSONMarshal(t *testing.T) {
	tests := []struct {
		name     string
		result   PSQLResult
		expected string
	}{
		{
			name:     "tabular output",
			result:   PSQLResult{Output: " id | name\n----+------\n  1 | test\n(1 row)\n"},
			expected: `{"output":" id | name\n----+------\n  1 | test\n(1 row)\n"}`,
		},
		{
			name:     "empty output",
			result:   PSQLResult{Output: ""},
			expected: `{"output":""}`,
		},
		{
			name:     "csv output from passthrough",
			result:   PSQLResult{Output: "id,name\n1,alice\n2,bob\n"},
			expected: `{"output":"id,name\n1,alice\n2,bob\n"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.result)
			require.NoError(t, err)
			require.JSONEq(t, tt.expected, string(b))
		})
	}
}

func TestPSQLResult_YAMLMarshal(t *testing.T) {
	result := PSQLResult{Output: "hello world\n"}

	b, err := yaml.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(b), "output:")
	require.Contains(t, string(b), "hello world")
}

func TestRunPSQLCommandForwardsSuccessfulDiagnostics(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("the subprocess fixture requires a POSIX shell")
	}
	tests := []struct {
		name        string
		script      string
		output      string
		diagnostics string
		err         string
	}{
		{"stdout and warning", `printf 'DO\n'; printf 'WARNING:  careful\n' >&2`, "DO\n", "WARNING:  careful\n", ""},
		{"notice only", `printf 'NOTICE:  fyi\n' >&2`, "", "NOTICE:  fyi\n", ""},
		{"stdout only", `printf '1\n'`, "1\n", "", ""},
		{"empty", `true`, "", "", ""},
		{"error keeps stderr in the error", `printf 'ERROR:  boom\n' >&2; exit 3`, "", "", "exit status 3: ERROR:  boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diagnostics bytes.Buffer
			result, err := runPSQLCommand(exec.Command("sh", "-c", tt.script), nil, &diagnostics)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.output, result.Output)
			}
			require.Equal(t, tt.diagnostics, diagnostics.String())
		})
	}
}

func TestRunPSQLCommandForwardsStdin(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("the subprocess fixture requires a POSIX shell")
	}
	csv := "id,body\n1,\"a, \"\"quoted\"\"\nline\"\n2,\"\u00e9\u6f22\"\n3,\"\"\n"
	tests := []struct {
		name   string
		script string
		stdin  string
		output string
		err    string
	}{
		{"exact bytes reach the child", `cat`, csv, csv, ""},
		{"empty stdin", `cat`, "", "", ""},
		{"a command that reads nothing", `printf 'SELECT 1\n'`, csv, "SELECT 1\n", ""},
		{"child error keeps stderr in the error", `cat >/dev/null; printf 'ERROR:  bad input\n' >&2; exit 1`, csv, "", "exit status 1: ERROR:  bad input"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diagnostics bytes.Buffer
			result, err := runPSQLCommand(exec.Command("sh", "-c", tt.script), strings.NewReader(tt.stdin), &diagnostics)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				require.Nil(t, result)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.output, result.Output)
			require.Empty(t, diagnostics.String())
		})
	}
}

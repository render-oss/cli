package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/sandbox"
)

func TestSandboxCopyAlias(t *testing.T) {
	cmd := newSandboxCopyCmd(nil)
	assert.True(t, cmd.HasAlias("cp"), "cp must alias copy")
}

// Every "render ..." line in a sandbox command's help must resolve to that same
// command. The singular "ea sandbox" is not an alias (see
// TestSandboxes_SingularFormRemoved), so an example using it is copy-pasteable
// but unrunnable, and with --help appended it prints ea's help and exits 0.
func TestSandboxHelpExamplesResolve(t *testing.T) {
	root := &cobra.Command{Use: "render"}
	earlyAccess := &cobra.Command{Use: "ea"}
	root.AddCommand(earlyAccess)
	setupSandboxCommands(earlyAccess, nil)

	sandboxes, _, err := root.Find([]string{"ea", "sandboxes"})
	require.NoError(t, err)
	require.NotEmpty(t, sandboxes.Commands())

	for _, cmd := range sandboxes.Commands() {
		t.Run(cmd.Name(), func(t *testing.T) {
			examples := exampleCommandLines(cmd.Long + "\n" + cmd.Example)
			require.NotEmpty(t, examples, "command documents no examples")

			for _, example := range examples {
				found, _, err := root.Find(commandPathArgs(example))
				require.NoError(t, err, "example %q", example)
				assert.Equal(t, cmd.CommandPath(), found.CommandPath(), "example %q does not run this command", example)
			}
		})
	}
}

// exampleCommandLines returns the "render ..." invocations in a help string.
func exampleCommandLines(help string) []string {
	var lines []string
	for _, line := range strings.Split(help, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "render ") {
			lines = append(lines, line)
		}
	}
	return lines
}

// commandPathArgs reduces an example to the args cobra resolves a command from:
// everything after "render" up to the first flag or "--" separator.
func commandPathArgs(example string) []string {
	fields := strings.Fields(example)[1:]
	for i, field := range fields {
		if strings.HasPrefix(field, "-") {
			return fields[:i]
		}
	}
	return fields
}

func TestSandboxCopyTextOutput(t *testing.T) {
	tests := []struct {
		name string
		out  *sandbox.CopyOut
		want string
	}{
		{
			name: "upload",
			out: &sandbox.CopyOut{Data: sandbox.CopyOutData{
				SandboxID:  "sbx-abc123",
				Direction:  sandbox.CopyDirectionUpload,
				LocalPath:  "./main.py",
				RemotePath: "main.py",
			}},
			want: "Uploaded ./main.py to sbx-abc123:main.py\n",
		},
		{
			name: "download",
			out: &sandbox.CopyOut{Data: sandbox.CopyOutData{
				SandboxID:  "sbx-abc123",
				Direction:  sandbox.CopyDirectionDownload,
				LocalPath:  "downloads/output.json",
				RemotePath: "output.json",
			}},
			want: "Downloaded sbx-abc123:output.json to downloads/output.json\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sandboxCopyTextOutput(tt.out))
		})
	}
}

// The path a download actually wrote is the field an agent reads out of JSON: a
// directory destination takes its name from the sandbox, so it is not a path the
// caller passed in.
func TestSandboxCopyJSONReportsWrittenPath(t *testing.T) {
	out, err := json.Marshal(&sandbox.CopyOut{Data: sandbox.CopyOutData{
		SandboxID:  "sbx-abc123",
		Direction:  sandbox.CopyDirectionDownload,
		LocalPath:  "downloads/output.json",
		RemotePath: "output.json",
	}})
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(out, &body))
	data := testrequire.SubMap(t, body, "data")
	assert.Equal(t, "downloads/output.json", data["localPath"])
	assert.Equal(t, "output.json", data["remotePath"])
	assert.Equal(t, "sbx-abc123", data["sandboxId"])
	assert.Equal(t, "download", data["direction"])
}

func TestParseSandboxCopyArgs(t *testing.T) {
	t.Run("upload direction", func(t *testing.T) {
		from, to, err := parseSandboxCopyArgs("./main.py", "sbx-abc123:/app/main.py")
		require.NoError(t, err)
		assert.Empty(t, from.sandboxID)
		assert.Equal(t, "./main.py", from.path)
		assert.Equal(t, "sbx-abc123", to.sandboxID)
		assert.Equal(t, "/app/main.py", to.path)
	})

	t.Run("download direction", func(t *testing.T) {
		from, to, err := parseSandboxCopyArgs("sbx-abc123:/app/out.json", "out.json")
		require.NoError(t, err)
		assert.Equal(t, "sbx-abc123", from.sandboxID)
		assert.Equal(t, "/app/out.json", from.path)
		assert.Empty(t, to.sandboxID)
		assert.Equal(t, "out.json", to.path)
	})

	t.Run("both local is an error", func(t *testing.T) {
		_, _, err := parseSandboxCopyArgs("a.txt", "b.txt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a sandbox path")
	})

	t.Run("both remote is an error", func(t *testing.T) {
		_, _, err := parseSandboxCopyArgs("sbx-a:/x", "sbx-b:/y")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not supported")
	})

	t.Run("relative sandbox path resolves under home", func(t *testing.T) {
		_, to, err := parseSandboxCopyArgs("a.txt", "sbx-abc123:app/main.py")
		require.NoError(t, err)
		assert.Equal(t, "app/main.py", to.path)
	})

	t.Run("dot-slash prefix is cleaned", func(t *testing.T) {
		_, to, err := parseSandboxCopyArgs("a.txt", "sbx-abc123:./main.py")
		require.NoError(t, err)
		assert.Equal(t, "main.py", to.path)
	})

	t.Run("empty sandbox path is the home directory", func(t *testing.T) {
		from, _, err := parseSandboxCopyArgs("sbx-abc123:", "./dump")
		require.NoError(t, err)
		assert.Equal(t, ".", from.path)
	})

	t.Run("escaping sandbox path is an error", func(t *testing.T) {
		_, _, err := parseSandboxCopyArgs("a.txt", "sbx-abc123:../up")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "escape")
	})

	t.Run("colon in a local path is not a remote", func(t *testing.T) {
		from, to, err := parseSandboxCopyArgs("./weird:name.txt", "sbx-abc123:/app/f")
		require.NoError(t, err)
		assert.Empty(t, from.sandboxID)
		assert.Equal(t, "./weird:name.txt", from.path)
		assert.Equal(t, "sbx-abc123", to.sandboxID)
	})
}

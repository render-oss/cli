package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEnvFiles_LaterOverridesAndUnions(t *testing.T) {
	dir := t.TempDir()

	writeEnv := func(name, contents string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(strings.TrimLeft(contents, "\n")), 0o600))
		return path
	}

	first := writeEnv("first.env", `
SHARED=from_first
ONLY_FIRST=a
OVERRIDDEN=v1
`)
	second := writeEnv("second.env", `
SHARED=from_second
ONLY_SECOND=b
OVERRIDDEN=v2
`)
	third := writeEnv("third.env", `
OVERRIDDEN=v3
ONLY_THIRD=c
`)

	vars, loaded, err := LoadEnvFiles([]string{first, second, third}, true)
	require.NoError(t, err)

	assert.Equal(t, []string{first, second, third}, loaded)
	assert.Equal(t, map[string]string{
		"SHARED":      "from_second",
		"ONLY_FIRST":  "a",
		"ONLY_SECOND": "b",
		"ONLY_THIRD":  "c",
		"OVERRIDDEN":  "v3",
	}, vars)
}

func TestEnvMapToKVStrings(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]string
		want []string
	}{
		{
			name: "nil map returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "empty map returns nil",
			in:   map[string]string{},
			want: nil,
		},
		{
			name: "single entry",
			in:   map[string]string{"FOO": "bar"},
			want: []string{"FOO=bar"},
		},
		{
			name: "multiple entries sorted by key",
			in: map[string]string{
				"GAMMA": "3",
				"ALPHA": "1",
				"BETA":  "2",
			},
			want: []string{"ALPHA=1", "BETA=2", "GAMMA=3"},
		},
		{
			name: "preserves empty values and special characters",
			in: map[string]string{
				"EMPTY":   "",
				"EQUALS":  "a=b=c",
				"SPACES":  "hello world",
				"UNICODE": "héllo",
			},
			want: []string{
				"EMPTY=",
				"EQUALS=a=b=c",
				"SPACES=hello world",
				"UNICODE=héllo",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EnvMapToKVStrings(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}

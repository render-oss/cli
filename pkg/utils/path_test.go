package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty unchanged", "", ""},
		{"non-tilde unchanged", "/abs/path", "/abs/path"},
		{"relative unchanged", "./foo", "./foo"},
		{"bare tilde", "~", home},
		{"tilde slash", "~/", filepath.Join(home, "")},
		{"tilde subpath", "~/foo/bar", filepath.Join(home, "foo/bar")},
		{"tilde backslash subpath", "~\\foo", filepath.Join(home, "foo")},
		{"tilde-user unchanged", "~alice/foo", "~alice/foo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExpandHome(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

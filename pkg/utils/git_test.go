package utils

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLooksLikeRemoteURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"https URL", "https://github.com/foo/bar", true},
		{"http URL", "http://example.com/repo.git", true},
		{"scp-style SSH", "git@github.com:foo/bar.git", true},
		{"ssh:// URL", "ssh://git@github.com/foo/bar.git", true},
		{"dot", ".", false},
		{"double dot", "..", false},
		{"relative path", "./foo", false},
		{"parent relative path", "../foo", false},
		{"absolute path", "/abs/path", false},
		{"home alias", "~", false},
		{"home subpath", "~/foo", false},
		{"bare name", "foo", false},
		{"two-segment name", "foo/bar", false},
		{"empty", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, LooksLikeRemoteURL(tc.input))
		})
	}
}

func TestNormalizeRemoteURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"https passthrough", "https://github.com/foo/bar.git", "https://github.com/foo/bar.git"},
		{"ssh:// passthrough", "ssh://git@github.com/foo/bar.git", "ssh://git@github.com/foo/bar.git"},
		{"scp-style github", "git@github.com:foo/bar.git", "https://github.com/foo/bar.git"},
		{"scp-style gitlab with nested group", "git@gitlab.com:group/sub/project.git", "https://gitlab.com/group/sub/project.git"},
		{"scp-style bitbucket", "git@bitbucket.org:owner/repo.git", "https://bitbucket.org/owner/repo.git"},
		{"malformed scp-style without path", "git@github.com:", "git@github.com:"},
		{"malformed scp-style without colon", "git@github.com", "git@github.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, NormalizeRemoteURL(tc.in))
		})
	}
}

func TestResolveLocalRepoURL(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantURL string
		wantErr string
	}{
		{
			name:    "passes through https URL",
			setup:   func(t *testing.T) string { return "https://github.com/foo/bar" },
			wantURL: "https://github.com/foo/bar",
		},
		{
			name:    "normalizes scp-style SSH URL to https",
			setup:   func(t *testing.T) string { return "git@github.com:foo/bar.git" },
			wantURL: "https://github.com/foo/bar.git",
		},
		{
			name:    "passes through ssh:// URL",
			setup:   func(t *testing.T) string { return "ssh://git@gitlab.com/foo/bar.git" },
			wantURL: "ssh://git@gitlab.com/foo/bar.git",
		},
		{
			name: "prefers origin when multiple remotes exist",
			setup: func(t *testing.T) string {
				return initRepoWithRemotes(t, map[string]string{
					"upstream": "https://github.com/upstream/repo.git",
					"origin":   "https://github.com/me/repo.git",
				})
			},
			wantURL: "https://github.com/me/repo.git",
		},
		{
			name: "falls back to first remote when no origin",
			setup: func(t *testing.T) string {
				return initRepoWithRemotes(t, map[string]string{
					"upstream": "https://github.com/upstream/repo.git",
				})
			},
			wantURL: "https://github.com/upstream/repo.git",
		},
		{
			name: "normalizes scp-style origin to https",
			setup: func(t *testing.T) string {
				return initRepoWithRemotes(t, map[string]string{
					"origin": "git@github.com:me/repo.git",
				})
			},
			wantURL: "https://github.com/me/repo.git",
		},
		{
			name: "detects .git when given a subdirectory",
			setup: func(t *testing.T) string {
				dir := initRepoWithRemotes(t, map[string]string{
					"origin": "https://github.com/me/repo.git",
				})
				sub := filepath.Join(dir, "pkg", "nested")
				require.NoError(t, os.MkdirAll(sub, 0o755))
				return sub
			},
			wantURL: "https://github.com/me/repo.git",
		},
		{
			name:    "resolves a bare directory name relative to cwd",
			setup:   setupRepoUnderCwd,
			wantURL: "https://github.com/me/myproject.git",
		},
		{
			name:    "errors when path does not exist",
			setup:   func(t *testing.T) string { return "/definitely/not/a/real/path/xyz-wor1221" },
			wantErr: "not a valid Git URL",
		},
		{
			name:    "errors when directory is not a git repo",
			setup:   func(t *testing.T) string { return t.TempDir() },
			wantErr: "no Git repository found",
		},
		{
			name: "errors when repo has no remotes",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				_, err := git.PlainInit(dir, false)
				require.NoError(t, err)
				return dir
			},
			wantErr: "no remotes",
		},
		{
			name: "errors when repo has remote but no commits",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				repo, err := git.PlainInit(dir, false)
				require.NoError(t, err)
				_, err = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/me/empty.git"}})
				require.NoError(t, err)
				return dir
			},
			wantErr: "no commits yet",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := tc.setup(t)
			got, err := ResolveLocalRepoURL(input)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantURL, got)
		})
	}
}

// setupRepoUnderCwd creates a git repo at <tmp>/myproject, chdirs to <tmp>, and
// returns the bare dir name "myproject".
func setupRepoUnderCwd(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	name := "myproject"
	full := filepath.Join(parent, name)
	require.NoError(t, os.MkdirAll(full, 0o755))
	_, err := git.PlainInit(full, false)
	require.NoError(t, err)
	repo, err := git.PlainOpen(full)
	require.NoError(t, err)
	_, err = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/me/myproject.git"}})
	require.NoError(t, err)
	commitInitial(t, repo, full)

	// NOTE: os.Chdir is process-global; do not add t.Parallel() to any test in
	// this file without replacing this.
	cwd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	require.NoError(t, os.Chdir(parent))

	return name
}

func initRepoWithRemotes(t *testing.T, remotes map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	for name, url := range remotes {
		_, err := repo.CreateRemote(&config.RemoteConfig{Name: name, URLs: []string{url}})
		require.NoError(t, err)
	}
	commitInitial(t, repo, dir)
	return dir
}

// commitInitial creates an initial commit so ResolveLocalRepoURL's HEAD check
// passes. Without it, freshly-init'd test repos look "empty" to the CLI.
func commitInitial(t *testing.T, repo *git.Repository, dir string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README"), []byte("test"), 0o644))
	w, err := repo.Worktree()
	require.NoError(t, err)
	_, err = w.Add("README")
	require.NoError(t, err)
	_, err = w.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@example.com", When: time.Now()},
	})
	require.NoError(t, err)
}

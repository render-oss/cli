package blueprint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"

	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
)

func newBlueprintGitRepo(t *testing.T) (string, *git.Repository) {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	require.NoError(t, repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/feature")))
	return dir, repo
}

func addBlueprintGitRemote(t *testing.T, repo *git.Repository, name, url string) {
	t.Helper()
	_, err := repo.CreateRemote(&config.RemoteConfig{Name: name, URLs: []string{url}})
	require.NoError(t, err)
}

func TestBuildCreateRequestLocalGitDefaults(t *testing.T) {
	t.Run("uses the local branch only when the repository is omitted", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			input CreateInput
			want  bptypes.BlueprintSource
		}{
			{
				name: "omitted repository uses origin",
				want: bptypes.BlueprintSource{Repo: "https://github.com/acme/app.git", Branch: new("feature")},
			},
			{
				name:  "explicit origin repository leaves branch to the API",
				input: CreateInput{Repo: new("https://github.com/acme/app")},
				want:  bptypes.BlueprintSource{Repo: "https://github.com/acme/app"},
			},
			{
				name:  "SSH scheme normalizes the host and transport",
				input: CreateInput{Repo: new("ssh://git@GITHUB.COM/acme/app.git")},
				want:  bptypes.BlueprintSource{Repo: "https://github.com/acme/app.git"},
			},
			{
				name:  "different repository leaves branch to the API",
				input: CreateInput{Repo: new("https://github.com/other/app")},
				want:  bptypes.BlueprintSource{Repo: "https://github.com/other/app"},
			},
			{
				name:  "explicit branch is preserved",
				input: CreateInput{Repo: new("https://github.com/acme/app"), Branch: new("release")},
				want:  bptypes.BlueprintSource{Repo: "https://github.com/acme/app", Branch: new("release")},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				dir, repo := newBlueprintGitRepo(t)
				addBlueprintGitRemote(t, repo, "origin", "git@github.com:acme/app.git")
				t.Chdir(dir)
				got, err := tc.input.buildCreateRequest(t.Context(), "workspace")
				require.NoError(t, err)
				tc.want.Path = new("render.yaml")
				require.Equal(t, tc.want, got.Source)
			})
		}
	})
}

func TestBuildCreateRequestExplicitRepositoryIgnoresOrigin(t *testing.T) {
	dir, repo := newBlueprintGitRepo(t)
	addBlueprintGitRemote(t, repo, "origin", "../local-mirror")
	t.Chdir(dir)

	input := CreateInput{Repo: new("https://github.com/acme/app")}
	got, err := input.buildCreateRequest(t.Context(), "workspace")
	require.NoError(t, err)
	require.Equal(t, *input.Repo, got.Source.Repo)
	require.Nil(t, got.Source.Branch)
}

func TestBuildCreateRequestOutsideAGitRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	input := CreateInput{Repo: new("https://github.com/acme/app")}
	got, err := input.buildCreateRequest(t.Context(), "workspace")
	require.NoError(t, err)
	require.Equal(t, *input.Repo, got.Source.Repo)
	require.Nil(t, got.Source.Branch)

	_, err = (CreateInput{}).buildCreateRequest(t.Context(), "workspace")
	require.ErrorContains(t, err, "cannot determine the repository from the local checkout; provide --repo")

	_, err = (CreateInput{Repo: new("https://github.com/acme/%gh")}).buildCreateRequest(t.Context(), "workspace")
	require.ErrorContains(t, err, "--repo must be a valid https:// or ssh:// URL")
}

func TestBuildCreateRequestRequiresOrigin(t *testing.T) {
	dir, repo := newBlueprintGitRepo(t)
	addBlueprintGitRemote(t, repo, "upstream", "https://github.com/acme/app")
	t.Chdir(dir)
	_, err := (CreateInput{}).buildCreateRequest(t.Context(), "workspace")
	require.ErrorContains(t, err, "cannot determine the repository from the local checkout; provide --repo")

	input := CreateInput{Repo: new("https://github.com/acme/app")}
	got, err := input.buildCreateRequest(t.Context(), "workspace")
	require.NoError(t, err)
	require.Equal(t, *input.Repo, got.Source.Repo)
	require.Nil(t, got.Source.Branch)
}

func TestBuildCreateRequestReportsMissingGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := (CreateInput{}).buildCreateRequest(t.Context(), "workspace")
	require.EqualError(t, err, "cannot determine the repository because git is not on PATH; provide --repo, or install Git")

	input := CreateInput{Repo: new("https://github.com/acme/app")}
	got, err := input.buildCreateRequest(t.Context(), "workspace")
	require.NoError(t, err)
	require.Equal(t, *input.Repo, got.Source.Repo)
}

func TestBuildCreateRequestRejectsHTTPOrigin(t *testing.T) {
	dir, repo := newBlueprintGitRepo(t)
	addBlueprintGitRemote(t, repo, "origin", "http://github.com/acme/app.git")
	t.Chdir(dir)

	_, err := (CreateInput{}).buildCreateRequest(t.Context(), "workspace")
	require.ErrorContains(t, err, "origin remote: provide an HTTPS or SSH repository URL")
}

func TestBuildCreateRequestDetachedHead(t *testing.T) {
	dir, repo := newBlueprintGitRepo(t)
	addBlueprintGitRemote(t, repo, "origin", "https://github.com/acme/app")
	require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, plumbing.NewHash("0123456789012345678901234567890123456789"))))
	t.Chdir(dir)

	_, err := (CreateInput{}).buildCreateRequest(t.Context(), "workspace")
	require.ErrorContains(t, err, "the local checkout has a detached HEAD; provide --branch")

	input := CreateInput{Branch: new("release")}
	got, err := input.buildCreateRequest(t.Context(), "workspace")
	require.NoError(t, err)
	require.Equal(t, bptypes.BlueprintSource{Repo: "https://github.com/acme/app", Branch: new("release"), Path: new("render.yaml")}, got.Source)

	input = CreateInput{Repo: new("https://github.com/acme/app")}
	got, err = input.buildCreateRequest(t.Context(), "workspace")
	require.NoError(t, err)
	require.Equal(t, *input.Repo, got.Source.Repo)
	require.Nil(t, got.Source.Branch)
}

func TestBuildCreateRequestBranchSharesNameWithTag(t *testing.T) {
	dir, repo := newBlueprintGitRepo(t)
	addBlueprintGitRemote(t, repo, "origin", "https://github.com/acme/app")
	require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference("refs/tags/feature", plumbing.NewHash("0123456789012345678901234567890123456789"))))
	t.Chdir(dir)

	request, err := (CreateInput{}).buildCreateRequest(t.Context(), "workspace")
	require.NoError(t, err)
	require.NotNil(t, request.Source.Branch)
	require.Equal(t, "feature", *request.Source.Branch)
}

func TestBuildCreateRequestFromSubdirectory(t *testing.T) {
	dir, repo := newBlueprintGitRepo(t)
	addBlueprintGitRemote(t, repo, "origin", "https://github.com/acme/app")
	subdir := filepath.Join(dir, "infra", "nested")
	require.NoError(t, os.MkdirAll(subdir, 0o755))
	t.Chdir(subdir)
	got, err := (CreateInput{Path: new("infra/render.yaml")}).buildCreateRequest(t.Context(), "workspace")
	require.NoError(t, err)
	require.Equal(t, bptypes.BlueprintSource{Repo: "https://github.com/acme/app", Branch: new("feature"), Path: new("infra/render.yaml")}, got.Source)
}

func TestBuildCreateRequestLinkedWorktree(t *testing.T) {
	dir, repo := newBlueprintGitRepo(t)
	addBlueprintGitRemote(t, repo, "origin", "https://github.com/acme/app")
	configPath := filepath.Join(dir, ".git", "config")
	contents, err := os.ReadFile(configPath)
	require.NoError(t, err)
	contents = append(contents, []byte("\n[extensions]\n\tworktreeConfig = true\n")...)
	require.NoError(t, os.WriteFile(configPath, contents, 0o644))
	worktree := t.TempDir()
	gitDir := filepath.Join(dir, ".git", "worktrees", "review")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	// Model a worktree's private HEAD and shared config with worktreeConfig enabled.
	require.NoError(t, os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "commondir"), []byte("../..\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/review\n"), 0o644))
	t.Chdir(worktree)
	got, err := (CreateInput{}).buildCreateRequest(t.Context(), "workspace")
	require.NoError(t, err)
	require.Equal(t, bptypes.BlueprintSource{Repo: "https://github.com/acme/app", Branch: new("review"), Path: new("render.yaml")}, got.Source)
}

package utils

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// ResolveLocalRepoURL turns a --repo-style value into a Git URL. If the
// value already looks like a URL it's returned (with SCP-style SSH URLs
// normalized to HTTPS so net/url can parse them). Otherwise the value is
// treated as a local directory: the function locates the .git for that
// path, picks the origin remote (or the first remote), and returns its
// URL. The repo must have at least one commit; otherwise the Render API
// would reject it later with a less helpful error.
//
// Errors are returned without a flag-specific prefix so callers can wrap
// them with the appropriate flag context (e.g. `--repo %q: %w`).
func ResolveLocalRepoURL(value string) (string, error) {
	if LooksLikeRemoteURL(value) {
		return NormalizeRemoteURL(value), nil
	}

	path, err := ExpandHome(value)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("not a valid Git URL and no directory exists at that path")
		}
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}

	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			return "", fmt.Errorf("no Git repository found at that path")
		}
		return "", err
	}

	remotes, err := repo.Remotes()
	if err != nil {
		return "", fmt.Errorf("failed to read remotes: %w", err)
	}
	if len(remotes) == 0 {
		return "", fmt.Errorf("repository has no remotes. Push the repo to GitHub, GitLab, or Bitbucket, then pass the URL")
	}

	pick := remotes[0]
	for _, r := range remotes {
		if r.Config().Name == "origin" {
			pick = r
			break
		}
	}
	urls := pick.Config().URLs
	if len(urls) == 0 {
		return "", fmt.Errorf("remote %q has no URL configured", pick.Config().Name)
	}

	if _, err := repo.Head(); err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return "", fmt.Errorf("the repository has no commits yet. Commit your changes and push to your Git provider, then re-run")
		}
		return "", fmt.Errorf("failed to read repository HEAD: %w", err)
	}

	return NormalizeRemoteURL(urls[0]), nil
}

// LooksLikeRemoteURL reports whether s is shaped like a remote Git URL
// (contains "://" or starts with "git@") rather than a local path.
func LooksLikeRemoteURL(s string) bool {
	return strings.Contains(s, "://") || strings.HasPrefix(s, "git@")
}

// NormalizeRemoteURL rewrites SCP-style SSH URLs (git@host:path) to
// https://host/path. The Render API parses repo URLs via net/url, which
// rejects SCP-style because the colon is read as a port separator. URLs
// with an explicit scheme are returned unchanged.
func NormalizeRemoteURL(u string) string {
	if strings.Contains(u, "://") {
		return u
	}
	if rest, ok := strings.CutPrefix(u, "git@"); ok {
		if host, path, found := strings.Cut(rest, ":"); found && host != "" && path != "" {
			return "https://" + host + "/" + path
		}
	}
	return u
}

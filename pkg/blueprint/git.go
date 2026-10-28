package blueprint

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
)

// getRemoteOrigin returns the local checkout's origin as an HTTPS repository URL.
func getRemoteOrigin(ctx context.Context) (string, error) {
	origin, err := executeGitCommand(ctx, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf(`cannot determine the repository from the local checkout; provide --repo, or check that this directory belongs to a Git repository with a remote named "origin": %w`, err)
	}
	originURL, err := normalizeGitRepoURL(origin)
	if err != nil {
		return "", fmt.Errorf("origin remote: %w", err)
	}
	return originURL, nil
}

// getLocalBranch returns the current checkout's branch, or an error for a detached HEAD.
func getLocalBranch(ctx context.Context) (string, error) {
	branch, err := executeGitCommand(ctx, "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("cannot determine the current branch from the local checkout; provide --branch, or use --repo to select a repository and its default branch: %w", err)
	}
	if branch == "" {
		return "", errors.New("the local checkout has a detached HEAD; provide --branch, or use --repo to select the repository's default branch")
	}
	return branch, nil
}

// executeGitCommand returns trimmed stdout and includes git's diagnostics in errors.
//
// Use native git for repository discovery because go-git v5.19.2 rejects
// extensions.worktreeConfig, including in linked worktrees, even with
// EnableDotGitCommonDir enabled. Its URL parser does not read repository
// configuration and is unaffected.
func executeGitCommand(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		// Preserve diagnostics as text without propagating git's process exit code.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("%s: %v", strings.TrimSpace(string(exitErr.Stderr)), err)
		}
		return "", errors.New(err.Error())
	}
	return strings.TrimSpace(string(out)), nil
}

// normalizeGitRepoURL converts SSH transport URLs to provider HTTPS URLs.
// Transport credentials are omitted; repository suffixes such as .git are kept.
func normalizeGitRepoURL(raw string) (string, error) {
	endpoint, err := transport.NewEndpoint(raw)
	if err != nil {
		return "", fmt.Errorf("parse repository URL: %w", err)
	}
	switch endpoint.Protocol {
	case "https":
	case "ssh":
		endpoint.Protocol = "https"
		endpoint.Port = 0
	default:
		return "", errors.New("provide an HTTPS or SSH repository URL")
	}
	if endpoint.Host == "" || strings.Trim(endpoint.Path, "/") == "" {
		return "", errors.New("repository URL must include a host and repository path")
	}
	endpoint.Host = strings.ToLower(endpoint.Host)
	endpoint.User = ""
	endpoint.Password = ""
	return endpoint.String(), nil
}

package github

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/go-github/v66/github"
)

// CreateRepoFromPath creates a GitHub repository from a local path (file or directory)
// and returns the repository URL
func CreateRepoFromPath(ctx context.Context, localPath string, repoName string, isPrivate bool, org string) (string, error) {
	// Read GitHub token
	token, err := readGitHubToken()
	if err != nil {
		return "", fmt.Errorf("failed to read GitHub token: %w", err)
	}

	// Create GitHub client
	client := github.NewClient(nil).WithAuthToken(token)

	// Get current user
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return "", fmt.Errorf("failed to get GitHub user: %w", err)
	}

	// Check if repo exists and find a unique name
	uniqueRepoName := repoName
	for i := 0; i < 100; i++ { // Try up to 100 times
		if i > 0 {
			uniqueRepoName = fmt.Sprintf("%s-%d", repoName, i)
		}
		
		// Check if repo exists
		if org != "" {
			_, resp, err := client.Repositories.Get(ctx, org, uniqueRepoName)
			if err != nil && resp.StatusCode == 404 {
				// Repo doesn't exist, we can use this name
				break
			}
		} else {
			_, resp, err := client.Repositories.Get(ctx, user.GetLogin(), uniqueRepoName)
			if err != nil && resp.StatusCode == 404 {
				// Repo doesn't exist, we can use this name
				break
			}
		}
	}

	// Create GitHub repository
	repo := &github.Repository{
		Name:    github.String(uniqueRepoName),
		Private: github.Bool(isPrivate),
	}

	// Use org if provided, otherwise create in user account
	createdRepo, _, err := client.Repositories.Create(ctx, org, repo)
	if err != nil {
		return "", fmt.Errorf("failed to create GitHub repository: %w", err)
	}

	// Create in-memory git repository
	fs := memfs.New()
	gitRepo, err := git.Init(memory.NewStorage(), fs)
	if err != nil {
		return "", fmt.Errorf("failed to init git repository: %w", err)
	}

	// Add files to the repository
	worktree, err := gitRepo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	// Check if path is a file or directory
	info, err := os.Stat(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat path: %w", err)
	}

	if info.IsDir() {
		// Add all files in directory
		err = addDirectoryToRepo(fs, worktree, localPath, "")
		if err != nil {
			return "", fmt.Errorf("failed to add directory to repo: %w", err)
		}
	} else {
		// Add single file
		err = addFileToRepo(fs, worktree, localPath, filepath.Base(localPath))
		if err != nil {
			return "", fmt.Errorf("failed to add file to repo: %w", err)
		}
	}

	// Check worktree status before commit
	status, err := worktree.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree status: %w", err)
	}
	
	// Check if there are any files to commit
	hasFiles := false
	for _, s := range status {
		if s.Staging != git.Unmodified {
			hasFiles = true
			break
		}
	}
	
	if !hasFiles {
		return "", fmt.Errorf("no files were added to the repository")
	}
	
	// Create initial commit
	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  user.GetName(),
			Email: user.GetEmail(),
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create commit: %w", err)
	}

	// Add remote
	_, err = gitRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{createdRepo.GetCloneURL()},
	})
	if err != nil {
		return "", fmt.Errorf("failed to add remote: %w", err)
	}

	// Push to GitHub
	err = gitRepo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth: &http.BasicAuth{
			Username: "github-token", // This can be anything except an empty string
			Password: token,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to push to GitHub: %w", err)
	}

	return createdRepo.GetCloneURL(), nil
}

func readGitHubToken() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	tokenPath := filepath.Join(homeDir, ".github-token")
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", fmt.Errorf("failed to read ~/.github-token: %w", err)
	}

	return strings.TrimSpace(string(tokenBytes)), nil
}

func addFileToRepo(fs billy.Filesystem, worktree *git.Worktree, sourcePath, destPath string) error {
	// Read file content
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}

	// Create file in memory filesystem
	file, err := fs.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(content)
	if err != nil {
		return err
	}

	// Add file to git
	_, err = worktree.Add(destPath)
	if err != nil {
		return fmt.Errorf("failed to add file to git: %w", err)
	}
	
	return nil
}

func addDirectoryToRepo(fs billy.Filesystem, worktree *git.Worktree, sourceDir, destDir string) error {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		sourcePath := filepath.Join(sourceDir, entry.Name())
		destPath := filepath.Join(destDir, entry.Name())

		// Skip .git directory
		if entry.Name() == ".git" {
			continue
		}

		if entry.IsDir() {
			// Create directory in memory filesystem
			err = fs.MkdirAll(destPath, 0755)
			if err != nil {
				return err
			}

			// Recursively add directory contents
			err = addDirectoryToRepo(fs, worktree, sourcePath, destPath)
			if err != nil {
				return err
			}
		} else {
			err = addFileToRepo(fs, worktree, sourcePath, destPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
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
func CreateRepoFromPath(ctx context.Context, localPath string, repoName string, isPrivate bool, generateDockerfile bool, org string) (string, error) {
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

	// Generate Dockerfile if requested
	if generateDockerfile {
		err = generateDockerfileInRepo(fs, worktree, localPath)
		if err != nil {
			return "", fmt.Errorf("failed to generate Dockerfile: %w", err)
		}
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
	return err
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

func generateDockerfileInRepo(fs billy.Filesystem, worktree *git.Worktree, localPath string) error {
	// Check if Dockerfile already exists
	if _, err := fs.Stat("Dockerfile"); err == nil {
		// Dockerfile already exists, don't overwrite
		return nil
	}

	// Detect the language/framework and generate appropriate Dockerfile
	dockerfile := detectAndGenerateDockerfile(localPath)
	
	// Create Dockerfile in memory filesystem
	file, err := fs.Create("Dockerfile")
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write([]byte(dockerfile))
	if err != nil {
		return err
	}

	// Add Dockerfile to git
	_, err = worktree.Add("Dockerfile")
	return err
}

func detectAndGenerateDockerfile(localPath string) string {
	// Check if it's a directory
	info, err := os.Stat(localPath)
	if err != nil || !info.IsDir() {
		// For single files or if we can't detect, use a generic Dockerfile
		return generateGenericDockerfile()
	}

	// Check for package.json (Node.js)
	if _, err := os.Stat(filepath.Join(localPath, "package.json")); err == nil {
		return generateNodeDockerfile()
	}

	// Check for requirements.txt (Python)
	if _, err := os.Stat(filepath.Join(localPath, "requirements.txt")); err == nil {
		return generatePythonDockerfile()
	}

	// Check for Gemfile (Ruby)
	if _, err := os.Stat(filepath.Join(localPath, "Gemfile")); err == nil {
		return generateRubyDockerfile()
	}

	// Check for go.mod (Go)
	if _, err := os.Stat(filepath.Join(localPath, "go.mod")); err == nil {
		return generateGoDockerfile()
	}

	// Default generic Dockerfile
	return generateGenericDockerfile()
}

func generateNodeDockerfile() string {
	return `FROM node:18-alpine

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci --only=production

# Copy application files
COPY . .

# Expose port (change if needed)
EXPOSE 3000

# Start the application
CMD ["npm", "start"]
`
}

func generatePythonDockerfile() string {
	return `FROM python:3.11-slim

WORKDIR /app

# Copy requirements
COPY requirements.txt .

# Install dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy application files
COPY . .

# Expose port (change if needed)
EXPOSE 8000

# Start the application (adjust command as needed)
CMD ["python", "app.py"]
`
}

func generateRubyDockerfile() string {
	return `FROM ruby:3.2-slim

WORKDIR /app

# Install dependencies
RUN apt-get update -qq && apt-get install -y build-essential

# Copy Gemfile
COPY Gemfile Gemfile.lock ./

# Install gems
RUN bundle install --without development test

# Copy application files
COPY . .

# Expose port (change if needed)
EXPOSE 3000

# Start the application (adjust command as needed)
CMD ["bundle", "exec", "ruby", "app.rb"]
`
}

func generateGoDockerfile() string {
	return `FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o main .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/main .

# Expose port (change if needed)
EXPOSE 8080

# Run the binary
CMD ["./main"]
`
}

func generateGenericDockerfile() string {
	return `FROM ubuntu:22.04

WORKDIR /app

# Copy application files
COPY . .

# Install basic dependencies (customize as needed)
RUN apt-get update && apt-get install -y \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Expose port (change if needed)
EXPOSE 8080

# Add your start command here
CMD ["/bin/bash"]
`
}
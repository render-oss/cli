package scaffold

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"gopkg.in/yaml.v3"
)

var repoURLs = map[Language]string{
	Python:     "https://github.com/render-examples/render-workflows-examples-python.git",
	TypeScript: "https://github.com/render-examples/render-workflows-examples-ts.git",
}

// localRepoEnvVars maps each language to an environment variable that, when
// set, points to a local checkout of the examples repo. This is used for
// development and testing of template.yaml files without publishing.
var localRepoEnvVars = map[Language]string{
	Python:     "RENDER_WORKFLOWS_TEMPLATES_PYTHON",
	TypeScript: "RENDER_WORKFLOWS_TEMPLATES_TYPESCRIPT",
}

// LocalRepoOverride returns the local templates directory for the given
// language if the corresponding environment variable is set and points to a
// valid directory. Returns an empty string otherwise.
func LocalRepoOverride(lang Language) string {
	envVar, ok := localRepoEnvVars[lang]
	if !ok {
		return ""
	}
	dir := os.Getenv(envVar)
	if dir == "" {
		return ""
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return ""
	}
	return dir
}

type Language string

const (
	Python     Language = "python"
	TypeScript Language = "node"
)

// DisplayName returns a human-friendly label for the language.
func (l Language) DisplayName() string {
	switch l {
	case Python:
		return "Python"
	case TypeScript:
		return "Node.js (TypeScript)"
	default:
		return string(l)
	}
}

// ExamplesRepoURL returns the browsable GitHub URL for the examples
// repository of the given language. Returns an empty string for unknown
// languages.
func ExamplesRepoURL(lang Language) string {
	url, ok := repoURLs[lang]
	if !ok {
		return ""
	}
	return strings.TrimSuffix(url, ".git")
}

func ParseLanguage(s string) (Language, error) {
	switch s {
	case "python", "py":
		return Python, nil
	case "node", "typescript", "ts":
		return TypeScript, nil
	default:
		return "", fmt.Errorf("unsupported language: %s (choose python or node)", s)
	}
}

// NextStep represents a single additional next-step instruction in the
// post-scaffold output. Steps 1 (install deps) and 2 (start dev server) are
// always generated from BuildCommand/StartCommand; these come after.
type NextStep struct {
	Label   string `yaml:"label"`
	Command string `yaml:"command,omitempty"`
	Hint    string `yaml:"hint,omitempty"`
}

// TemplateMetadata holds metadata parsed from a template's template.yaml file.
type TemplateMetadata struct {
	Name          string `yaml:"name"`
	DirName       string `yaml:"-"` // directory name on disk
	Description   string `yaml:"description"`
	Default       bool   `yaml:"default"`
	WorkflowsRoot string `yaml:"workflowsRoot,omitempty"`
	ClientRoot    string `yaml:"clientRoot,omitempty"`

	// Preferred field names for build/start commands
	RenderBuildCommand string `yaml:"renderBuildCommand,omitempty"`
	RenderStartCommand string `yaml:"renderStartCommand,omitempty"`
	// Legacy field names (used if renderBuildCommand/renderStartCommand are empty)
	BuildCommand string `yaml:"buildCommand,omitempty"`
	StartCommand string `yaml:"startCommand,omitempty"`

	NextSteps []NextStep `yaml:"nextSteps,omitempty"`
	// LegacyNextSteps supports the old "additionalNextSteps" key name
	// during the transition period.
	LegacyNextSteps []NextStep `yaml:"additionalNextSteps,omitempty"`
}

// DiscoveredTemplate is the slim subset of TemplateMetadata returned by
// DiscoverTemplates – only the fields the CLI needs for template selection.
type DiscoveredTemplate struct {
	Name        string
	DirName     string
	Description string
	Default     bool
}

type Options struct {
	Language     Language
	TemplateName string // directory name of the template (e.g. "hello-world")
	Dir          string
	RepoDir      string // path to the cloned workflow-examples repo
}

type Result struct {
	Dir                string
	Files              []string
	Language           Language
	BuildCommand       string // local version (venv pip, venv python, etc.)
	StartCommand       string // local version (venv python, etc.)
	RenderBuildCommand string // original from template, for deploying to Render
	RenderStartCommand string // original from template, for deploying to Render
	WorkflowsRoot      string
	ClientRoot         string
	NextSteps          []NextStep
}

// CloneTemplatesRepo performs a shallow clone of the examples repo for the
// given language into destDir. destDir must be an empty or newly created
// directory (e.g. from os.MkdirTemp); cloning into a non-empty directory
// will fail.
//
// The provided context controls the clone timeout. Cancelling the context
// will abort the network operation.
func CloneTemplatesRepo(ctx context.Context, destDir string, lang Language) error {
	url, ok := repoURLs[lang]
	if !ok {
		return fmt.Errorf("no templates repository for language %s", lang)
	}

	entries, err := os.ReadDir(destDir)
	if err != nil {
		return fmt.Errorf("failed to read destination directory: %w", err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("destination directory %s is not empty", destDir)
	}

	cloneOpts := &git.CloneOptions{
		URL:   url,
		Depth: 1,
	}

	if _, err := git.PlainCloneContext(ctx, destDir, false, cloneOpts); err != nil {
		return fmt.Errorf("failed to clone workflow templates repository: %w", err)
	}

	return nil
}

// DiscoverTemplates scans root-level subdirectories in the cloned repo for
// template.yaml files and returns the information the CLI needs for template
// selection (name, description, directory name, default flag).
func DiscoverTemplates(repoDir string) ([]DiscoveredTemplate, error) {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}

	var templates []DiscoveredTemplate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip hidden directories (.git, etc.)
		if entry.Name()[0] == '.' {
			continue
		}

		metaPath := filepath.Join(repoDir, entry.Name(), "template.yaml")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			// Skip directories without a template.yaml
			continue
		}

		var meta TemplateMetadata
		if err := yaml.Unmarshal(data, &meta); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: invalid template.yaml\n", entry.Name())
			continue
		}

		name := meta.Name
		if name == "" {
			name = entry.Name()
		}
		templates = append(templates, DiscoveredTemplate{
			Name:        name,
			DirName:     entry.Name(),
			Description: meta.Description,
			Default:     meta.Default,
		})
	}

	if len(templates) == 0 {
		return nil, fmt.Errorf("no templates found in repository for chosen language")
	}

	// Sort: default template first, then alphabetically by DirName.
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].Default != templates[j].Default {
			return templates[i].Default
		}
		return templates[i].DirName < templates[j].DirName
	})

	return templates, nil
}

// Scaffold copies template files from a cloned repo to the output directory.
// It reads template.yaml for metadata and copies all other files.
func Scaffold(opts Options) (*Result, error) {
	templateDir := filepath.Join(opts.RepoDir, opts.TemplateName)
	if _, err := os.Stat(templateDir); err != nil {
		return nil, fmt.Errorf("template %q not found for language %s", opts.TemplateName, opts.Language)
	}

	// Parse template.yaml for metadata
	metaPath := filepath.Join(templateDir, "template.yaml")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template.yaml: %w", err)
	}

	var meta TemplateMetadata
	if err := yaml.Unmarshal(metaData, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse template.yaml: %w", err)
	}

	// Create output directory
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Copy all files except template.yaml
	var createdFiles []string
	err = filepath.WalkDir(templateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks to prevent path traversal
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		rel, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}

		// Skip root dir and template.yaml
		if rel == "." {
			return nil
		}
		if rel == "template.yaml" {
			return nil
		}

		destPath := filepath.Join(opts.Dir, rel)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		if err := copyFile(path, destPath); err != nil {
			return fmt.Errorf("failed to copy %s: %w", rel, err)
		}
		createdFiles = append(createdFiles, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Support the old "additionalNextSteps" key as a fallback
	nextSteps := meta.NextSteps
	if len(nextSteps) == 0 && len(meta.LegacyNextSteps) > 0 {
		nextSteps = meta.LegacyNextSteps
	}

	// Resolve build/start commands: prefer renderBuildCommand/renderStartCommand,
	// fall back to buildCommand/startCommand for older templates.
	renderBuild := meta.RenderBuildCommand
	if renderBuild == "" {
		renderBuild = meta.BuildCommand
	}
	renderStart := meta.RenderStartCommand
	if renderStart == "" {
		renderStart = meta.StartCommand
	}

	return &Result{
		Dir:                opts.Dir,
		Files:              createdFiles,
		Language:           opts.Language,
		BuildCommand:       LocalBuildCommand(opts.Language, renderBuild),
		StartCommand:       LocalStartCommand(opts.Language, renderStart),
		RenderBuildCommand: renderBuild,
		RenderStartCommand: renderStart,
		WorkflowsRoot:      meta.WorkflowsRoot,
		ClientRoot:         meta.ClientRoot,
		NextSteps:          nextSteps,
	}, nil
}

// copyFile copies a single file, preserving permissions.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// InitGitRepo initializes a new git repository in dir.
func InitGitRepo(dir string) error {
	_, err := git.PlainInit(dir, false)
	return err
}

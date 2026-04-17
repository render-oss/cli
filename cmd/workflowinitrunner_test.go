package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/v2/pkg/workflows/scaffold"
)

type mockInitDeps struct {
	localRepoOverrideFn func(lang scaffold.Language) string
	cloneRepoFn         func(ctx context.Context, destDir string, lang scaffold.Language) error
	discoverTemplatesFn func(repoDir string) ([]scaffold.DiscoveredTemplate, error)
	scaffoldFn          func(opts scaffold.Options) (*scaffold.Result, error)
	installDepsFn       func(ctx context.Context, dir string, installCmd string) error
	initGitFn           func(dir string) error
}

func (m *mockInitDeps) LocalRepoOverride(lang scaffold.Language) string {
	if m.localRepoOverrideFn != nil {
		return m.localRepoOverrideFn(lang)
	}
	return ""
}

func (m *mockInitDeps) CloneRepo(ctx context.Context, destDir string, lang scaffold.Language) error {
	if m.cloneRepoFn != nil {
		return m.cloneRepoFn(ctx, destDir, lang)
	}
	return nil
}

func (m *mockInitDeps) DiscoverTemplates(repoDir string) ([]scaffold.DiscoveredTemplate, error) {
	if m.discoverTemplatesFn != nil {
		return m.discoverTemplatesFn(repoDir)
	}
	return []scaffold.DiscoveredTemplate{{Name: "Hello World", DirName: "hello-world", Default: true}}, nil
}

func (m *mockInitDeps) Scaffold(opts scaffold.Options) (*scaffold.Result, error) {
	if m.scaffoldFn != nil {
		return m.scaffoldFn(opts)
	}
	return &scaffold.Result{
		Dir:            opts.Dir,
		Language:       opts.Language,
		BuildCommand: "pip install -r requirements.txt",
		StartCommand: "python main.py",
	}, nil
}

func (m *mockInitDeps) InstallDeps(ctx context.Context, dir string, installCmd string) error {
	if m.installDepsFn != nil {
		return m.installDepsFn(ctx, dir, installCmd)
	}
	return nil
}

func (m *mockInitDeps) InitGit(dir string) error {
	if m.initGitFn != nil {
		return m.initGitFn(dir)
	}
	return nil
}

// newNonInteractiveRunner creates a runner in non-interactive mode with captured output.
func newNonInteractiveRunner(deps InitDeps) (*WorkflowInitRunner, *bytes.Buffer) {
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	return &WorkflowInitRunner{
		deps:        deps,
		interactive: false,
		cmd:         cmd,
	}, &buf
}

func TestRunnerNonInteractive_RequiresLanguage(t *testing.T) {
	runner, _ := newNonInteractiveRunner(&mockInitDeps{})

	err := runner.Run(context.Background(), WorkflowInitInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--language is required")
}

func TestRunnerNonInteractive_CloneRepoErrorPropagates(t *testing.T) {
	deps := &mockInitDeps{
		cloneRepoFn: func(ctx context.Context, destDir string, lang scaffold.Language) error {
			return errors.New("network timeout")
		},
	}
	runner, _ := newNonInteractiveRunner(deps)

	dir := t.TempDir() + "/output"
	err := runner.Run(context.Background(), WorkflowInitInput{Language: "python", Dir: dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "network timeout")
}

func TestRunnerNonInteractive_DiscoverTemplatesErrorPropagates(t *testing.T) {
	deps := &mockInitDeps{
		discoverTemplatesFn: func(repoDir string) ([]scaffold.DiscoveredTemplate, error) {
			return nil, errors.New("no templates found")
		},
	}
	runner, _ := newNonInteractiveRunner(deps)

	dir := t.TempDir() + "/output"
	err := runner.Run(context.Background(), WorkflowInitInput{Language: "python", Dir: dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no templates found")
}

func TestRunnerNonInteractive_ScaffoldCalledWithCorrectOptions(t *testing.T) {
	var capturedOpts scaffold.Options
	deps := &mockInitDeps{
		scaffoldFn: func(opts scaffold.Options) (*scaffold.Result, error) {
			capturedOpts = opts
			return &scaffold.Result{
				Dir:          opts.Dir,
				Language:     opts.Language,
				BuildCommand: "pip install",
				StartCommand: "python main.py",
			}, nil
		},
	}
	runner, _ := newNonInteractiveRunner(deps)

	dir := t.TempDir() + "/output"
	err := runner.Run(context.Background(), WorkflowInitInput{
		Language: "python",
		Template: "hello-world",
		Dir:      dir,
	})
	require.NoError(t, err)

	assert.Equal(t, scaffold.Python, capturedOpts.Language)
	assert.Equal(t, "hello-world", capturedOpts.TemplateName)
}

func TestRunnerNonInteractive_DefaultsToFirstTemplate(t *testing.T) {
	var capturedOpts scaffold.Options
	deps := &mockInitDeps{
		discoverTemplatesFn: func(repoDir string) ([]scaffold.DiscoveredTemplate, error) {
			return []scaffold.DiscoveredTemplate{
				{Name: "Advanced", DirName: "advanced", Default: true},
				{Name: "Basic", DirName: "basic"},
			}, nil
		},
		scaffoldFn: func(opts scaffold.Options) (*scaffold.Result, error) {
			capturedOpts = opts
			return &scaffold.Result{
				Dir:          opts.Dir,
				Language:     opts.Language,
				BuildCommand: "pip install",
				StartCommand: "python main.py",
			}, nil
		},
	}
	runner, _ := newNonInteractiveRunner(deps)

	dir := t.TempDir() + "/output"
	err := runner.Run(context.Background(), WorkflowInitInput{
		Language: "python",
		Dir:      dir,
	})
	require.NoError(t, err)
	assert.Equal(t, "advanced", capturedOpts.TemplateName)
}

func TestRunnerNonInteractive_InstallDepsRewritesPipForPython(t *testing.T) {
	var capturedInstallCmd string
	deps := &mockInitDeps{
		scaffoldFn: func(opts scaffold.Options) (*scaffold.Result, error) {
			return &scaffold.Result{
				Dir:          opts.Dir,
				Language:     opts.Language,
				BuildCommand: "pip install -r requirements.txt",
				StartCommand: "python main.py",
			}, nil
		},
		installDepsFn: func(ctx context.Context, dir string, installCmd string) error {
			capturedInstallCmd = installCmd
			return nil
		},
	}
	runner, _ := newNonInteractiveRunner(deps)

	dir := t.TempDir() + "/output"
	err := runner.Run(context.Background(), WorkflowInitInput{
		Language:    "python",
		Dir:         dir,
		InstallDeps: true,
	})
	require.NoError(t, err)
	// Should create venv and use venv's pip
	assert.Contains(t, capturedInstallCmd, "-m venv .venv")
	assert.Contains(t, capturedInstallCmd, "pip install -r requirements.txt")
}

func TestRunnerNonInteractive_InstallDepsErrorIsFatal(t *testing.T) {
	deps := &mockInitDeps{
		installDepsFn: func(ctx context.Context, dir string, installCmd string) error {
			return errors.New("pip not found")
		},
	}
	runner, _ := newNonInteractiveRunner(deps)

	dir := t.TempDir() + "/output"
	err := runner.Run(context.Background(), WorkflowInitInput{
		Language:    "python",
		Dir:         dir,
		InstallDeps: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pip not found")
}

func TestRunnerNonInteractive_GitInitErrorIsFatal(t *testing.T) {
	deps := &mockInitDeps{
		initGitFn: func(dir string) error {
			return errors.New("git not installed")
		},
	}
	runner, _ := newNonInteractiveRunner(deps)

	dir := t.TempDir() + "/output"
	err := runner.Run(context.Background(), WorkflowInitInput{
		Language: "python",
		Dir:      dir,
		Git:      true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git not installed")
}

func TestRunnerNonInteractive_GitSummaryUsesGitNaming(t *testing.T) {
	runner, output := newNonInteractiveRunner(&mockInitDeps{})

	dir := t.TempDir() + "/output"
	err := runner.Run(context.Background(), WorkflowInitInput{
		Language: "python",
		Dir:      dir,
		Git:      true,
	})
	require.NoError(t, err)

	assert.Contains(t, output.String(), "Initialized Git repository")
	assert.NotContains(t, output.String(), "Initialized git repository")
}

func TestRunnerNonInteractive_LocalRepoOverrideSkipsClone(t *testing.T) {
	cloneCalled := false
	deps := &mockInitDeps{
		localRepoOverrideFn: func(lang scaffold.Language) string {
			return "/fake/local/repo"
		},
		cloneRepoFn: func(ctx context.Context, destDir string, lang scaffold.Language) error {
			cloneCalled = true
			return errors.New("should not be called")
		},
		discoverTemplatesFn: func(repoDir string) ([]scaffold.DiscoveredTemplate, error) {
			assert.Equal(t, "/fake/local/repo", repoDir)
			return []scaffold.DiscoveredTemplate{{Name: "Test", DirName: "test", Default: true}}, nil
		},
	}
	runner, _ := newNonInteractiveRunner(deps)

	dir := t.TempDir() + "/output"
	err := runner.Run(context.Background(), WorkflowInitInput{
		Language: "python",
		Dir:      dir,
	})
	require.NoError(t, err)
	assert.False(t, cloneCalled, "CloneRepo should not be called when local override is set")
}

func TestRunnerNonInteractive_NonEmptyDirReturnsError(t *testing.T) {
	deps := &mockInitDeps{}
	runner, _ := newNonInteractiveRunner(deps)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(dir+"/existing.txt", []byte("hello"), 0o644))

	err := runner.Run(context.Background(), WorkflowInitInput{
		Language: "python",
		Dir:      dir,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists and is not empty")
}

package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLanguage(t *testing.T) {
	tests := []struct {
		input   string
		want    Language
		wantErr bool
	}{
		{"python", Python, false},
		{"py", Python, false},
		{"node", TypeScript, false},
		{"typescript", TypeScript, false},
		{"ts", TypeScript, false},
		{"go", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLanguage(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// setupFakeRepo creates a fake cloned repo structure for testing.
// Templates are root-level subdirectories, matching the real repo layout.
func setupFakeRepo(t *testing.T, templateName string, meta string, files map[string]string) string {
	t.Helper()
	repoDir := t.TempDir()
	templateDir := filepath.Join(repoDir, templateName)

	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "template.yaml"), []byte(meta), 0o644))

	for path, content := range files {
		fullPath := filepath.Join(templateDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
	}

	return repoDir
}

// addTemplate adds an additional template directory to an existing repo dir.
func addTemplate(t *testing.T, repoDir, templateName, meta string) {
	t.Helper()
	dir := filepath.Join(repoDir, templateName)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "template.yaml"), []byte(meta), 0o644))
}

func TestDiscoverTemplates(t *testing.T) {
	// Create two template directories at the repo root.
	// "hello-world" is the default; "advanced" is not.
	repoDir := setupFakeRepo(t, "hello-world",
		"name: Hello World\ndescription: Simple greeting\ndefault: true\n", nil)
	addTemplate(t, repoDir, "advanced",
		"name: Advanced\ndescription: Advanced example\n")

	templates, err := DiscoverTemplates(repoDir)
	require.NoError(t, err)
	require.Len(t, templates, 2)

	// Default template sorts first, then alphabetically
	assert.Equal(t, "hello-world", templates[0].DirName)
	assert.True(t, templates[0].Default)
	assert.Equal(t, "advanced", templates[1].DirName)
	assert.Equal(t, "Hello World", templates[0].Name)
	assert.Equal(t, "Simple greeting", templates[0].Description)
}

func TestDiscoverTemplatesEmpty(t *testing.T) {
	repoDir := t.TempDir()
	_, err := DiscoverTemplates(repoDir)
	require.Error(t, err)
}

func TestDiscoverTemplatesSkipsDirsWithoutYAML(t *testing.T) {
	repoDir := setupFakeRepo(t, "valid", "name: Valid\n", nil)

	// Add a directory without template.yaml
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "invalid"), 0o755))

	templates, err := DiscoverTemplates(repoDir)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	assert.Equal(t, "valid", templates[0].DirName)
}

func TestDiscoverTemplatesFields(t *testing.T) {
	meta := `name: Hello World
description: Simple task + subtask basics
default: true
workflowsRoot: .
clientRoot: client
buildCommand: pip install -r requirements.txt
startCommand: render-workflows main:app
nextSteps:
  - label: "Run a task"
    command: "render workflows taskruns start hello --local --input='[\"world\"]'"
    hint: "If you see \"Hello, world!\" your tasks are working!"
  - label: Connect your app
    hint: See README.md for Client SDK integration snippets.
`
	repoDir := setupFakeRepo(t, "hello-world", meta, map[string]string{
		"main.py": "print('hello')",
	})

	templates, err := DiscoverTemplates(repoDir)
	require.NoError(t, err)
	require.Len(t, templates, 1)

	tm := templates[0]
	assert.Equal(t, "Hello World", tm.Name)
	assert.Equal(t, "Simple task + subtask basics", tm.Description)
	assert.True(t, tm.Default)
	assert.Equal(t, "hello-world", tm.DirName)
}

func TestScaffoldPreservesPlaceholders(t *testing.T) {
	meta := `name: Test
buildCommand: pip install
startCommand: python app.py
nextSteps:
  - label: "Run"
    command: "{{startCommand}}"
    hint: "After {{buildCommand}}"
`
	repoDir := setupFakeRepo(t, "test", meta, map[string]string{
		"app.py": "print('hi')\n",
	})

	outDir := filepath.Join(t.TempDir(), "workflows")
	result, err := Scaffold(Options{
		Language:     Python,
		TemplateName: "test",
		Dir:          outDir,
		RepoDir:      repoDir,
	})
	require.NoError(t, err)

	// Scaffold should NOT interpolate — the CLI layer handles styled replacement.
	assert.Equal(t, "{{startCommand}}", result.NextSteps[0].Command)
	assert.Equal(t, "After {{buildCommand}}", result.NextSteps[0].Hint)
}

func TestScaffoldPython(t *testing.T) {
	meta := `name: Hello World
default: true
buildCommand: pip install -r requirements.txt
startCommand: render-workflows main:app
nextSteps:
  - label: Run a task
    command: render workflows taskruns start hello --local
`
	repoDir := setupFakeRepo(t, "hello-world", meta, map[string]string{
		"main.py":          "print('hello')\n",
		"requirements.txt": "render-sdk>=0.5.0\n",
		"README.md":        "# Hello World\n",
	})

	outDir := filepath.Join(t.TempDir(), "workflows")
	result, err := Scaffold(Options{
		Language:     Python,
		TemplateName: "hello-world",
		Dir:          outDir,
		RepoDir:      repoDir,
	})
	require.NoError(t, err)

	assert.Equal(t, Python, result.Language)
	// Local commands are rewritten for the venv
	assert.Contains(t, result.BuildCommand, "pip install -r requirements.txt")
	assert.Contains(t, result.StartCommand, "render-workflows main:app")
	// Render commands preserve the original template values
	assert.Equal(t, "pip install -r requirements.txt", result.RenderBuildCommand)
	assert.Equal(t, "render-workflows main:app", result.RenderStartCommand)
	require.Len(t, result.NextSteps, 1)

	expectedFiles := []string{"README.md", "main.py", "requirements.txt", ".gitignore", ".env.example"}
	require.Len(t, result.Files, len(expectedFiles))
	for _, f := range expectedFiles {
		assert.Contains(t, result.Files, f)

		info, err := os.Stat(filepath.Join(outDir, f))
		require.NoError(t, err, "expected file %s to exist", f)
		assert.NotZero(t, info.Size(), "file %s should not be empty", f)
	}
}

func TestScaffoldTypeScript(t *testing.T) {
	meta := `name: Hello World
buildCommand: npm install
startCommand: npx tsx src/index.ts
`
	repoDir := setupFakeRepo(t, "hello-world", meta, map[string]string{
		"src/index.ts":  "console.log('hello')\n",
		"package.json":  `{"name":"workflows"}\n`,
		"tsconfig.json": `{"compilerOptions":{}}\n`,
		"README.md":     "# Hello World\n",
	})

	outDir := filepath.Join(t.TempDir(), "workflows")
	result, err := Scaffold(Options{
		Language:     TypeScript,
		TemplateName: "hello-world",
		Dir:          outDir,
		RepoDir:      repoDir,
	})
	require.NoError(t, err)

	assert.Equal(t, TypeScript, result.Language)
	assert.Equal(t, "npx tsx src/index.ts", result.StartCommand)

	expectedFiles := []string{"README.md", "package.json", "src/index.ts", "tsconfig.json", ".gitignore", ".env.example"}
	require.Len(t, result.Files, len(expectedFiles))
	for _, f := range expectedFiles {
		assert.Contains(t, result.Files, f)

		info, err := os.Stat(filepath.Join(outDir, f))
		require.NoError(t, err, "expected file %s to exist", f)
		assert.NotZero(t, info.Size(), "file %s should not be empty", f)
	}

	// Verify src/ subdirectory was created
	info, err := os.Stat(filepath.Join(outDir, "src"))
	require.NoError(t, err, "src dir should exist")
	assert.True(t, info.IsDir())
}

func TestScaffoldExcludesTemplateYAML(t *testing.T) {
	meta := `name: Test
buildCommand: test-install
`
	repoDir := setupFakeRepo(t, "test", meta, map[string]string{
		"main.py": "print('hello')\n",
	})

	outDir := filepath.Join(t.TempDir(), "workflows")
	result, err := Scaffold(Options{
		Language:     Python,
		TemplateName: "test",
		Dir:          outDir,
		RepoDir:      repoDir,
	})
	require.NoError(t, err)

	assert.NotContains(t, result.Files, "template.yaml")

	_, err = os.Stat(filepath.Join(outDir, "template.yaml"))
	assert.True(t, os.IsNotExist(err), "template.yaml should not exist in output directory")
}

func TestScaffoldCreatesSubdirectories(t *testing.T) {
	meta := `name: Test
buildCommand: test-install
`
	repoDir := setupFakeRepo(t, "test", meta, map[string]string{
		"src/index.ts": "console.log('hello')\n",
	})

	outDir := filepath.Join(t.TempDir(), "deeply", "nested", "workflows")
	_, err := Scaffold(Options{
		Language:     TypeScript,
		TemplateName: "test",
		Dir:          outDir,
		RepoDir:      repoDir,
	})
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(outDir, "src"))
	require.NoError(t, err, "src dir should exist")
	assert.True(t, info.IsDir())
}

func TestScaffoldMissingTemplate(t *testing.T) {
	repoDir := t.TempDir()

	outDir := filepath.Join(t.TempDir(), "workflows")
	_, err := Scaffold(Options{
		Language:     Python,
		TemplateName: "nonexistent",
		Dir:          outDir,
		RepoDir:      repoDir,
	})
	require.Error(t, err)
}

func TestScaffoldFileContent(t *testing.T) {
	meta := `name: Test
buildCommand: test-install
`
	expectedContent := "from render_sdk import Workflows\n\napp = Workflows()\n"
	repoDir := setupFakeRepo(t, "test", meta, map[string]string{
		"main.py": expectedContent,
	})

	outDir := filepath.Join(t.TempDir(), "workflows")
	_, err := Scaffold(Options{
		Language:     Python,
		TemplateName: "test",
		Dir:          outDir,
		RepoDir:      repoDir,
	})
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(outDir, "main.py"))
	require.NoError(t, err)
	assert.Equal(t, expectedContent, string(got))
}

func TestScaffoldCreatesGitignore(t *testing.T) {
	tests := []struct {
		name   string
		lang   Language
		wantIn []string
	}{
		{"Python", Python, []string{"dist/", ".env", ".venv/", "__pycache__/"}},
		{"TypeScript", TypeScript, []string{"dist/", ".env", "node_modules/"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := "name: Test\nbuildCommand: test\n"
			repoDir := setupFakeRepo(t, "test", meta, map[string]string{
				"app.txt": "hello\n",
			})

			outDir := filepath.Join(t.TempDir(), "workflows")
			_, err := Scaffold(Options{
				Language:     tt.lang,
				TemplateName: "test",
				Dir:          outDir,
				RepoDir:      repoDir,
			})
			require.NoError(t, err)

			got, err := os.ReadFile(filepath.Join(outDir, ".gitignore"))
			require.NoError(t, err)
			for _, entry := range tt.wantIn {
				assert.Contains(t, string(got), entry)
			}
		})
	}
}

func TestScaffoldCreatesEnvExample(t *testing.T) {
	meta := "name: Test\nbuildCommand: test\n"
	repoDir := setupFakeRepo(t, "test", meta, map[string]string{
		"app.txt": "hello\n",
	})

	outDir := filepath.Join(t.TempDir(), "workflows")
	_, err := Scaffold(Options{
		Language:     Python,
		TemplateName: "test",
		Dir:          outDir,
		RepoDir:      repoDir,
	})
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(outDir, ".env.example"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "RENDER_API_KEY=")
}

func TestScaffoldDoesNotOverwriteTemplateFiles(t *testing.T) {
	meta := "name: Test\nbuildCommand: test\n"
	repoDir := setupFakeRepo(t, "test", meta, map[string]string{
		"app.txt":      "hello\n",
		".gitignore":   "custom-ignore\n",
		".env.example": "CUSTOM_VAR=foo\n",
	})

	outDir := filepath.Join(t.TempDir(), "workflows")
	_, err := Scaffold(Options{
		Language:     Python,
		TemplateName: "test",
		Dir:          outDir,
		RepoDir:      repoDir,
	})
	require.NoError(t, err)

	gotGitignore, err := os.ReadFile(filepath.Join(outDir, ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, "custom-ignore\n", string(gotGitignore))

	gotEnv, err := os.ReadFile(filepath.Join(outDir, ".env.example"))
	require.NoError(t, err)
	assert.Equal(t, "CUSTOM_VAR=foo\n", string(gotEnv))
}

func TestInitGitRepoUsesMainAsDefaultBranch(t *testing.T) {
	dir := t.TempDir()

	err := InitGitRepo(dir)
	require.NoError(t, err)

	repo, err := git.PlainOpen(dir)
	require.NoError(t, err)

	head, err := repo.Reference(plumbing.HEAD, false)
	require.NoError(t, err)
	assert.Equal(t, plumbing.SymbolicReference, head.Type())
	assert.Equal(t, plumbing.Main, head.Target())
}

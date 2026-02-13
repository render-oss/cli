package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsRenderSkill(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"render-deploy", true},
		{"render-debug", true},
		{"render-monitor", true},
		{"render-migrate-from-heroku", true},
		{"render", true},
		{"not-render", false},
		{"my-skill", false},
		{"renderer", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isRenderSkill(tt.name))
		})
	}
}

func TestHashSkillDir(t *testing.T) {
	t.Run("deterministic hash", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Test skill"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("readme"), 0o644))

		hash1, err := HashSkillDir(dir)
		require.NoError(t, err)

		hash2, err := HashSkillDir(dir)
		require.NoError(t, err)

		assert.Equal(t, hash1, hash2)
		assert.Len(t, hash1, 64) // SHA-256 hex string
	})

	t.Run("changes when content changes", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("v1"), 0o644))

		hash1, err := HashSkillDir(dir)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("v2"), 0o644))

		hash2, err := HashSkillDir(dir)
		require.NoError(t, err)

		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("changes when file is renamed", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("content"), 0o644))

		hash1, err := HashSkillDir(dir)
		require.NoError(t, err)

		require.NoError(t, os.Rename(filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")))

		hash2, err := HashSkillDir(dir)
		require.NoError(t, err)

		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()

		hash, err := HashSkillDir(dir)
		require.NoError(t, err)
		assert.Len(t, hash, 64)
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		_, err := HashSkillDir("/nonexistent/path")
		assert.Error(t, err)
	})
}

func TestParseSkillFrontmatter(t *testing.T) {
	t.Run("parses name and description", func(t *testing.T) {
		dir := t.TempDir()
		content := `---
name: render-deploy
description: Deploy applications to Render.
metadata:
  version: "1.2.0"
---
# Skill content here
`
		path := filepath.Join(dir, "SKILL.md")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		info := parseSkillFrontmatter(path)
		assert.Equal(t, "render-deploy", info.Name)
		assert.Equal(t, "Deploy applications to Render.", info.Description)
		assert.Equal(t, "1.2.0", info.Version())
	})

	t.Run("returns unknown version when missing", func(t *testing.T) {
		dir := t.TempDir()
		content := `---
name: render-debug
description: Debug stuff.
---
# Content
`
		path := filepath.Join(dir, "SKILL.md")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		info := parseSkillFrontmatter(path)
		assert.Equal(t, "render-debug", info.Name)
		assert.Equal(t, "unknown", info.Version())
	})

	t.Run("returns empty on missing file", func(t *testing.T) {
		info := parseSkillFrontmatter("/nonexistent/SKILL.md")
		assert.Equal(t, "", info.Name)
	})

	t.Run("returns empty on invalid frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "SKILL.md")
		require.NoError(t, os.WriteFile(path, []byte("no frontmatter here"), 0o644))

		info := parseSkillFrontmatter(path)
		assert.Equal(t, "", info.Name)
	})
}

func TestToInstalled(t *testing.T) {
	info := SkillInfo{
		Name:        "render-deploy",
		Description: "Deploy apps.",
		Metadata:    SkillsMetadata{Version: "1.0.0"},
	}

	installed := info.ToInstalled("abc123hash")

	assert.Equal(t, "render-deploy", installed.Name)
	assert.Equal(t, "1.0.0", installed.Version)
	assert.Equal(t, "abc123hash", installed.Hash)
}

func TestToolNames(t *testing.T) {
	tools := []Tool{
		{Name: "Claude Code", SkillsDir: "/a"},
		{Name: "Cursor", SkillsDir: "/b"},
	}
	assert.Equal(t, []string{"Claude Code", "Cursor"}, ToolNames(tools))
}

func TestFilterTools(t *testing.T) {
	tools := []Tool{
		{Name: "Claude Code", SkillsDir: "/a"},
		{Name: "Cursor", SkillsDir: "/b"},
		{Name: "Codex, OpenCode, and others", SkillsDir: "/c"},
	}

	t.Run("matches by name", func(t *testing.T) {
		result := FilterTools(tools, "cursor")
		require.Len(t, result, 1)
		assert.Equal(t, "Cursor", result[0].Name)
	})

	t.Run("no match", func(t *testing.T) {
		result := FilterTools(tools, "vscode")
		assert.Empty(t, result)
	})
}

func TestRemoveSkills(t *testing.T) {
	t.Run("removes selected skills", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "render-deploy"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "render-debug"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "render-monitor"), 0o755))

		err := RemoveSkills(dir, []string{"render-deploy", "render-debug"})
		require.NoError(t, err)

		entries, _ := os.ReadDir(dir)
		assert.Len(t, entries, 1)
		assert.Equal(t, "render-monitor", entries[0].Name())
	})

	t.Run("ignores nonexistent skills", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "render-deploy"), 0o755))

		err := RemoveSkills(dir, []string{"render-nonexistent"})
		require.NoError(t, err)

		entries, _ := os.ReadDir(dir)
		assert.Len(t, entries, 1)
	})

	t.Run("handles nonexistent directory", func(t *testing.T) {
		err := RemoveSkills("/nonexistent/path", []string{"render-deploy"})
		assert.NoError(t, err)
	})
}

func TestDetectInstalledSkills(t *testing.T) {
	t.Run("finds render skills with SKILL.md", func(t *testing.T) {
		dir := t.TempDir()

		// Render skill with frontmatter.
		skillDir := filepath.Join(dir, "render-deploy")
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: render-deploy
description: Deploy stuff.
metadata:
  version: "2.0.0"
---
`), 0o644))

		// Non-render dir — should be ignored.
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "my-custom-skill"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "my-custom-skill", "SKILL.md"), []byte("---\nname: custom\n---\n"), 0o644))

		// Render dir without SKILL.md — should be ignored.
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "render-broken"), 0o755))

		found := DetectInstalledSkills(dir)
		require.Len(t, found, 1)
		assert.Equal(t, "render-deploy", found[0].Name)
		assert.Equal(t, "2.0.0", found[0].Version())
	})

	t.Run("returns nil for empty directory", func(t *testing.T) {
		dir := t.TempDir()
		found := DetectInstalledSkills(dir)
		assert.Nil(t, found)
	})

	t.Run("returns nil for nonexistent directory", func(t *testing.T) {
		found := DetectInstalledSkills("/nonexistent")
		assert.Nil(t, found)
	})
}

func TestScanInstalledState(t *testing.T) {
	t.Run("discovers skills across tools", func(t *testing.T) {
		tool1 := t.TempDir()
		tool2 := t.TempDir()

		// Same skill in both tools — should deduplicate.
		for _, dir := range []string{tool1, tool2} {
			skillDir := filepath.Join(dir, "render-deploy")
			require.NoError(t, os.MkdirAll(skillDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: render-deploy\n---\n"), 0o644))
		}

		// Extra skill in tool1 only.
		debugDir := filepath.Join(tool1, "render-debug")
		require.NoError(t, os.MkdirAll(debugDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(debugDir, "SKILL.md"), []byte("---\nname: render-debug\n---\n"), 0o644))

		tools := []Tool{
			{Name: "Tool1", SkillsDir: tool1},
			{Name: "Tool2", SkillsDir: tool2},
		}

		installed, toolNames, warnings := ScanInstalledState(tools)

		assert.Equal(t, []string{"Tool1", "Tool2"}, toolNames)
		assert.Len(t, installed, 2) // deduplicated
		assert.Empty(t, warnings)

		names := make(map[string]bool)
		for _, sk := range installed {
			names[sk.Name] = true
			assert.NotEmpty(t, sk.Hash)
		}
		assert.True(t, names["render-deploy"])
		assert.True(t, names["render-debug"])
	})

	t.Run("returns empty for tools with no skills", func(t *testing.T) {
		dir := t.TempDir()
		tools := []Tool{{Name: "Empty", SkillsDir: dir}}

		installed, toolNames, warnings := ScanInstalledState(tools)
		assert.Empty(t, installed)
		assert.Empty(t, toolNames)
		assert.Empty(t, warnings)
	})
}

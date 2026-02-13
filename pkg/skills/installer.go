package skills

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"gopkg.in/yaml.v3"
)

const (
	repoHTTPS = "https://github.com/render-oss/skills.git"
)

// Tool represents an AI coding tool that supports skills.
type Tool struct {
	Name      string
	SkillsDir string
}

// DetectTools scans for known AI coding tool directories and returns those
// that are present on the system. If the parent config directory exists but
// the skills subdirectory does not, it is created automatically.
func DetectTools() ([]Tool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine home directory: %w", err)
	}

	candidates := []struct {
		name      string
		parentDir string
		skillsDir string
	}{
		// ~/.agents/skills is a shared skill directory read globally by Codex
		// and OpenCode. Other agents may adopt this path in the future.
		// See https://developers.openai.com/codex/skills
		// and https://opencode.ai/docs/skills/
		{"Codex, OpenCode, and others", home, filepath.Join(home, ".agents", "skills")},

		// The following agents use their own global skill directories.
		{"AdaL", filepath.Join(home, ".adal"), filepath.Join(home, ".adal", "skills")},
		{"Amp", filepath.Join(home, ".config", "amp"), filepath.Join(home, ".config", "agents", "skills")},
		{"Antigravity", filepath.Join(home, ".gemini", "antigravity"), filepath.Join(home, ".gemini", "antigravity", "skills")},
		{"Augment", filepath.Join(home, ".augment"), filepath.Join(home, ".augment", "skills")},
		{"Claude Code", filepath.Join(home, ".claude"), filepath.Join(home, ".claude", "skills")},
		{"Cline", filepath.Join(home, ".cline"), filepath.Join(home, ".cline", "skills")},
		{"CodeBuddy", filepath.Join(home, ".codebuddy"), filepath.Join(home, ".codebuddy", "skills")},
		{"Command Code", filepath.Join(home, ".commandcode"), filepath.Join(home, ".commandcode", "skills")},
		{"Continue", filepath.Join(home, ".continue"), filepath.Join(home, ".continue", "skills")},
		{"Crush", filepath.Join(home, ".config", "crush"), filepath.Join(home, ".config", "crush", "skills")},
		{"Cursor", filepath.Join(home, ".cursor"), filepath.Join(home, ".cursor", "skills")},
		{"Droid", filepath.Join(home, ".factory"), filepath.Join(home, ".factory", "skills")},
		{"Gemini CLI", filepath.Join(home, ".gemini"), filepath.Join(home, ".gemini", "skills")},
		{"GitHub Copilot", filepath.Join(home, ".copilot"), filepath.Join(home, ".copilot", "skills")},
		{"Goose", filepath.Join(home, ".config", "goose"), filepath.Join(home, ".config", "goose", "skills")},
		{"iFlow CLI", filepath.Join(home, ".iflow"), filepath.Join(home, ".iflow", "skills")},
		{"Junie", filepath.Join(home, ".junie"), filepath.Join(home, ".junie", "skills")},
		{"Kilo Code", filepath.Join(home, ".kilocode"), filepath.Join(home, ".kilocode", "skills")},
		{"Kiro CLI", filepath.Join(home, ".kiro"), filepath.Join(home, ".kiro", "skills")},
		{"Kode", filepath.Join(home, ".kode"), filepath.Join(home, ".kode", "skills")},
		{"MCPJam", filepath.Join(home, ".mcpjam"), filepath.Join(home, ".mcpjam", "skills")},
		{"Mistral Vibe", filepath.Join(home, ".vibe"), filepath.Join(home, ".vibe", "skills")},
		{"Mux", filepath.Join(home, ".mux"), filepath.Join(home, ".mux", "skills")},
		{"Neovate", filepath.Join(home, ".neovate"), filepath.Join(home, ".neovate", "skills")},
		{"OpenClaw", filepath.Join(home, ".openclaw"), filepath.Join(home, ".openclaw", "skills")},
		{"OpenHands", filepath.Join(home, ".openhands"), filepath.Join(home, ".openhands", "skills")},
		{"Pi", filepath.Join(home, ".pi", "agent"), filepath.Join(home, ".pi", "agent", "skills")},
		{"Pochi", filepath.Join(home, ".pochi"), filepath.Join(home, ".pochi", "skills")},
		{"Qoder", filepath.Join(home, ".qoder"), filepath.Join(home, ".qoder", "skills")},
		{"Qwen Code", filepath.Join(home, ".qwen"), filepath.Join(home, ".qwen", "skills")},
		{"Roo Code", filepath.Join(home, ".roo"), filepath.Join(home, ".roo", "skills")},
		{"Trae", filepath.Join(home, ".trae"), filepath.Join(home, ".trae", "skills")},
		{"Trae CN", filepath.Join(home, ".trae-cn"), filepath.Join(home, ".trae-cn", "skills")},
		{"Windsurf", filepath.Join(home, ".codeium", "windsurf"), filepath.Join(home, ".codeium", "windsurf", "skills")},
		{"Zencoder", filepath.Join(home, ".zencoder"), filepath.Join(home, ".zencoder", "skills")},
	}

	var tools []Tool
	for _, c := range candidates {
		if !dirExists(c.parentDir) {
			continue
		}

		if !dirExists(c.skillsDir) {
			if err := os.MkdirAll(c.skillsDir, 0o755); err != nil {
				continue
			}
		}

		tools = append(tools, Tool{
			Name:      c.name,
			SkillsDir: c.skillsDir,
		})
	}

	return tools, nil
}

// FilterTools returns only the tools whose names contain the given filter
// string (case-insensitive). Useful for the --tool flag.
func FilterTools(tools []Tool, filter string) []Tool {
	filter = strings.ToLower(filter)
	var filtered []Tool
	for _, t := range tools {
		if strings.Contains(strings.ToLower(t.Name), filter) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// CloneSkillsRepo performs a shallow clone of the render-oss/skills repo into
// the given directory. The default branch is always used.
func CloneSkillsRepo(destDir string) error {
	opts := &git.CloneOptions{
		URL:   repoHTTPS,
		Depth: 1,
	}

	if _, err := git.PlainClone(destDir, false, opts); err != nil {
		return fmt.Errorf("failed to clone skills repository: %w", err)
	}

	return nil
}

// SkillInfo holds metadata parsed from a skill's SKILL.md frontmatter.
type SkillInfo struct {
	Name        string         `yaml:"name"`
	DirName     string         `yaml:"-"` // actual directory name on disk
	Description string         `yaml:"description"`
	Metadata    SkillsMetadata `yaml:"metadata"`
}

// SkillsMetadata holds optional metadata fields from SKILL.md frontmatter.
type SkillsMetadata struct {
	Version string `yaml:"version"`
}

// Version returns the skill's version string, or "unknown" if not set.
func (s SkillInfo) Version() string {
	if s.Metadata.Version == "" {
		return "unknown"
	}
	return s.Metadata.Version
}

// ToInstalled converts a SkillInfo to an InstalledSkill for state persistence.
// The hash parameter is the content hash of the skill directory.
func (s SkillInfo) ToInstalled(hash string) InstalledSkill {
	return InstalledSkill{
		Name:    s.Name,
		DirName: s.DirName,
		Version: s.Version(),
		Hash:    hash,
	}
}

// ToInstalledWithScope converts a SkillInfo to an InstalledSkill with a specific scope.
func (s SkillInfo) ToInstalledWithScope(hash string, scope Scope) InstalledSkill {
	return InstalledSkill{
		Name:    s.Name,
		DirName: s.DirName,
		Version: s.Version(),
		Hash:    hash,
		Scope:   scope,
	}
}

// ToolNames returns the display names of the given tools.
func ToolNames(tools []Tool) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}

// InstallResult holds the outcome of a skills install operation.
type InstallResult struct {
	Skills []SkillInfo `json:"skills"`
	Tools  []Tool      `json:"tools"`
	DryRun bool        `json:"dry_run"`
}

// InstallSelectedSkills copies skill directories from sourceDir/skills/* into
// toolDir. If selectedSkills is non-empty, only skills whose directory names
// match an entry in the list are installed. It removes any previously installed
// render skill directories first. Returns metadata for each installed skill.
func InstallSelectedSkills(toolDir, sourceDir string, selectedSkills []string) ([]SkillInfo, error) {
	skillsSrc := filepath.Join(sourceDir, "skills")
	if !dirExists(skillsSrc) {
		return nil, fmt.Errorf("skills directory not found in cloned repo: %s", skillsSrc)
	}

	// Remove old render skill installations.
	// When a selection filter is provided, only remove the skills being replaced
	// to avoid wiping unrelated skills (e.g. during a partial update).
	if err := removeOldSkills(toolDir, selectedSkills); err != nil {
		return nil, fmt.Errorf("failed to remove old skills: %w", err)
	}

	// Build a set for quick lookup when filtering.
	selected := make(map[string]bool, len(selectedSkills))
	for _, s := range selectedSkills {
		selected[s] = true
	}

	entries, err := os.ReadDir(skillsSrc)
	if err != nil {
		return nil, fmt.Errorf("failed to read skills source directory: %w", err)
	}

	var installed []SkillInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip hidden/special directories.
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}

		// If a selection filter is provided, skip skills not in the list.
		if len(selected) > 0 && !selected[name] {
			continue
		}

		srcPath := filepath.Join(skillsSrc, name)
		skillMD := filepath.Join(srcPath, "SKILL.md")

		// Only install directories that contain a SKILL.md.
		if _, err := os.Stat(skillMD); os.IsNotExist(err) {
			continue
		}

		destPath := filepath.Join(toolDir, name)
		if err := copyDir(srcPath, destPath); err != nil {
			return installed, fmt.Errorf("failed to copy skill %s: %w", name, err)
		}

		info := parseSkillFrontmatter(skillMD)
		info.DirName = name
		if info.Name == "" {
			info.Name = name
		}
		installed = append(installed, info)
	}

	return installed, nil
}

// ReadSkillsFromRepo reads skill metadata from a cloned repo without installing.
// Useful for dry-run previews.
func ReadSkillsFromRepo(sourceDir string) []SkillInfo {
	skillsSrc := filepath.Join(sourceDir, "skills")
	entries, err := os.ReadDir(skillsSrc)
	if err != nil {
		return nil
	}

	var skills []SkillInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}

		skillMD := filepath.Join(skillsSrc, name, "SKILL.md")
		if _, err := os.Stat(skillMD); os.IsNotExist(err) {
			continue
		}

		info := parseSkillFrontmatter(skillMD)
		info.DirName = name
		if info.Name == "" {
			info.Name = name
		}
		skills = append(skills, info)
	}
	return skills
}

// parseSkillFrontmatter extracts YAML frontmatter from a SKILL.md file.
// Frontmatter is delimited by --- on its own line at the start of the file.
func parseSkillFrontmatter(path string) SkillInfo {
	f, err := os.Open(path)
	if err != nil {
		return SkillInfo{}
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// First line must be "---".
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return SkillInfo{}
	}

	var sb strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	if scanner.Err() != nil {
		return SkillInfo{}
	}

	var info SkillInfo
	if err := yaml.Unmarshal([]byte(sb.String()), &info); err != nil {
		return SkillInfo{}
	}
	return info
}

// ShortenPath replaces the home directory prefix with ~ for display.
func ShortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if runtime.GOOS == "windows" {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// HashSkillDir computes a deterministic SHA-256 hash of a skill directory's
// contents. Files are sorted by relative path so the result is consistent
// regardless of OS walk order. Both file paths and contents are fed into the
// hash so renames and content changes are detected.
func HashSkillDir(dirPath string) (string, error) {
	var paths []string
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk skill directory: %w", err)
	}

	sort.Strings(paths)

	h := sha256.New()
	for _, rel := range paths {
		// Include the path so renames change the hash.
		h.Write([]byte(rel))

		data, err := os.ReadFile(filepath.Join(dirPath, rel))
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", rel, err)
		}
		h.Write(data)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// isRenderSkill returns true if the directory name matches the render skill
// naming convention (render-* or render).
func isRenderSkill(name string) bool {
	return strings.HasPrefix(name, "render-") || name == "render"
}

// DetectInstalledSkills scans a tool's skills directory for existing render
// skill directories and reads their SKILL.md frontmatter.
// This is used as a fallback when no state file exists.
func DetectInstalledSkills(toolDir string) []SkillInfo {
	entries, err := os.ReadDir(toolDir)
	if err != nil {
		return nil
	}

	var found []SkillInfo
	for _, entry := range entries {
		if !entry.IsDir() || !isRenderSkill(entry.Name()) {
			continue
		}

		skillMD := filepath.Join(toolDir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillMD); os.IsNotExist(err) {
			continue
		}

		info := parseSkillFrontmatter(skillMD)
		info.DirName = entry.Name()
		if info.Name == "" {
			info.Name = entry.Name()
		}
		found = append(found, info)
	}
	return found
}

// ScanInstalledState builds a SkillsState from skills found on disk across all
// detected tools at both user and project scope. Each discovered skill gets a
// content hash so subsequent update checks can detect changes. Use this as a
// fallback when no state file exists. Any hashing warnings are returned so
// callers can log them.
func ScanInstalledState(tools []Tool) (installed []InstalledSkill, toolNames []string, warnings []string) {
	seen := make(map[string]bool)
	toolSeen := make(map[string]bool)

	// Scan user scope (home directory tools)
	for _, t := range tools {
		found := DetectInstalledSkills(t.SkillsDir)
		if len(found) == 0 {
			continue
		}
		if !toolSeen[t.Name] {
			toolSeen[t.Name] = true
			toolNames = append(toolNames, t.Name)
		}
		for _, s := range found {
			key := "user:" + s.DirName
			if seen[key] {
				continue
			}
			seen[key] = true
			hash, err := HashSkillDir(filepath.Join(t.SkillsDir, s.DirName))
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("could not hash %s: %s", s.Name, err))
			}
			installed = append(installed, s.ToInstalledWithScope(hash, ScopeUser))
		}
	}

	// Scan project scope (repo root tools)
	repoRoot, err := GetRepoRoot()
	if err == nil {
		for _, t := range tools {
			projectDir := GetScopedSkillsDir(t, ScopeProject, repoRoot)
			found := DetectInstalledSkills(projectDir)
			if len(found) == 0 {
				continue
			}
			if !toolSeen[t.Name] {
				toolSeen[t.Name] = true
				toolNames = append(toolNames, t.Name)
			}
			for _, s := range found {
				key := "project:" + s.DirName
				if seen[key] {
					continue
				}
				seen[key] = true
				hash, err := HashSkillDir(filepath.Join(projectDir, s.DirName))
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("could not hash %s (project): %s", s.Name, err))
				}
				installed = append(installed, s.ToInstalledWithScope(hash, ScopeProject))
			}
		}
	}

	return installed, toolNames, warnings
}

// removeOldSkills deletes previously installed render skill directories from
// the target tool directory. If only is non-empty, only directories whose
// names match an entry in the list are removed. If only is empty, all
// render-* skill directories are removed.
func removeOldSkills(toolDir string, only []string) error {
	entries, err := os.ReadDir(toolDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	onlySet := make(map[string]bool, len(only))
	for _, name := range only {
		onlySet[name] = true
	}

	for _, entry := range entries {
		if !entry.IsDir() || !isRenderSkill(entry.Name()) {
			continue
		}
		if len(onlySet) > 0 && !onlySet[entry.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(toolDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// RemoveSkills removes specific skill directories by name from a tool's
// skills directory. Unlike removeOldSkills, this only removes the named
// skills rather than all render-* directories.
func RemoveSkills(toolDir string, skillNames []string) error {
	nameSet := make(map[string]bool, len(skillNames))
	for _, n := range skillNames {
		nameSet[n] = true
	}

	entries, err := os.ReadDir(toolDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() || !nameSet[entry.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(toolDir, entry.Name())); err != nil {
			return fmt.Errorf("failed to remove %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		return copyFile(path, destPath)
	})
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

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

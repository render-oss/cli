package skills

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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
		{"Claude Code (global)", filepath.Join(home, ".claude"), filepath.Join(home, ".claude", "skills")},
		{"Codex", filepath.Join(home, ".codex"), filepath.Join(home, ".codex", "skills")},
		{"OpenCode", filepath.Join(home, ".config", "opencode"), filepath.Join(home, ".config", "opencode", "skills")},
		{"Cursor", filepath.Join(home, ".cursor"), filepath.Join(home, ".cursor", "skills")},
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
// the given directory. Requires git to be installed.
func CloneSkillsRepo(destDir string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is required but not found in PATH: %w", err)
	}

	cmd := exec.Command("git", "clone", "--quiet", "--depth", "1", repoHTTPS, destDir)

	// Suppress interactive git prompts.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clone skills repository: %w\n%s", err, string(output))
	}

	return nil
}

// SkillInfo holds metadata parsed from a skill's SKILL.md frontmatter.
type SkillInfo struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// InstallSkills copies skill directories from sourceDir/skills/* into toolDir.
// It removes any previously installed render skill directories first.
// Returns metadata for each installed skill.
func InstallSkills(toolDir, sourceDir string) ([]SkillInfo, error) {
	skillsSrc := filepath.Join(sourceDir, "skills")
	if !dirExists(skillsSrc) {
		return nil, fmt.Errorf("skills directory not found in cloned repo: %s", skillsSrc)
	}

	// Remove old render skill installations.
	if err := removeOldSkills(toolDir); err != nil {
		return nil, fmt.Errorf("failed to remove old skills: %w", err)
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

// removeOldSkills deletes previously installed render skill directories from
// the target tool directory.
func removeOldSkills(toolDir string) error {
	entries, err := os.ReadDir(toolDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "render-") || name == "render" {
			if err := os.RemoveAll(filepath.Join(toolDir, name)); err != nil {
				return err
			}
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

package skills

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Scope represents where skills are installed.
type Scope string

const (
	// ScopeUser installs skills to the user's home directory (~/.{tool}/skills/).
	// These are available to the current user across all repositories.
	ScopeUser Scope = "user"

	// ScopeProject installs skills to the repository root (./.{tool}/skills/).
	// These are committed to git and available to all collaborators.
	ScopeProject Scope = "project"
)

// IsValid returns true if the scope is a recognized value.
func (s Scope) IsValid() bool {
	return s == ScopeUser || s == ScopeProject
}

// String returns the string representation of the scope.
func (s Scope) String() string {
	return string(s)
}

// ParseScope converts a string to a Scope, returning an error if invalid.
func ParseScope(s string) (Scope, error) {
	scope := Scope(strings.ToLower(s))
	if !scope.IsValid() {
		return "", fmt.Errorf("invalid scope %q: must be 'user' or 'project'", s)
	}
	return scope, nil
}

// GetRepoRoot returns the root directory of the current git repository.
// Returns an error if not in a git repository.
func GetRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// ToolConfig holds the configuration for a specific AI coding tool.
type ToolConfig struct {
	Name      string // Display name (e.g., "Claude Code")
	ParentDir string // Parent directory name (e.g., ".claude")
}

// knownTools maps tool names to their parent directory configurations.
// This is used to determine the project-scope skills directory.
var knownTools = map[string]ToolConfig{
	"Codex, OpenCode, and others": {Name: "Codex, OpenCode, and others", ParentDir: ".agents"},
	"AdaL":                        {Name: "AdaL", ParentDir: ".adal"},
	"Amp":                         {Name: "Amp", ParentDir: ".config/agents"},
	"Antigravity":                 {Name: "Antigravity", ParentDir: ".gemini/antigravity"},
	"Augment":                     {Name: "Augment", ParentDir: ".augment"},
	"Claude Code":                 {Name: "Claude Code", ParentDir: ".claude"},
	"Cline":                       {Name: "Cline", ParentDir: ".cline"},
	"CodeBuddy":                   {Name: "CodeBuddy", ParentDir: ".codebuddy"},
	"Command Code":                {Name: "Command Code", ParentDir: ".commandcode"},
	"Continue":                    {Name: "Continue", ParentDir: ".continue"},
	"Crush":                       {Name: "Crush", ParentDir: ".config/crush"},
	"Cursor":                      {Name: "Cursor", ParentDir: ".cursor"},
	"Droid":                       {Name: "Droid", ParentDir: ".factory"},
	"Gemini CLI":                  {Name: "Gemini CLI", ParentDir: ".gemini"},
	"GitHub Copilot":              {Name: "GitHub Copilot", ParentDir: ".copilot"},
	"Goose":                       {Name: "Goose", ParentDir: ".config/goose"},
	"iFlow CLI":                   {Name: "iFlow CLI", ParentDir: ".iflow"},
	"Junie":                       {Name: "Junie", ParentDir: ".junie"},
	"Kilo Code":                   {Name: "Kilo Code", ParentDir: ".kilocode"},
	"Kiro CLI":                    {Name: "Kiro CLI", ParentDir: ".kiro"},
	"Kode":                        {Name: "Kode", ParentDir: ".kode"},
	"MCPJam":                      {Name: "MCPJam", ParentDir: ".mcpjam"},
	"Mistral Vibe":                {Name: "Mistral Vibe", ParentDir: ".vibe"},
	"Mux":                         {Name: "Mux", ParentDir: ".mux"},
	"Neovate":                     {Name: "Neovate", ParentDir: ".neovate"},
	"OpenClaw":                    {Name: "OpenClaw", ParentDir: ".openclaw"},
	"OpenHands":                   {Name: "OpenHands", ParentDir: ".openhands"},
	"Pi":                          {Name: "Pi", ParentDir: ".pi/agent"},
	"Pochi":                       {Name: "Pochi", ParentDir: ".pochi"},
	"Qoder":                       {Name: "Qoder", ParentDir: ".qoder"},
	"Qwen Code":                   {Name: "Qwen Code", ParentDir: ".qwen"},
	"Roo Code":                    {Name: "Roo Code", ParentDir: ".roo"},
	"Trae":                        {Name: "Trae", ParentDir: ".trae"},
	"Trae CN":                     {Name: "Trae CN", ParentDir: ".trae-cn"},
	"Windsurf":                    {Name: "Windsurf", ParentDir: ".codeium/windsurf"},
	"Zencoder":                    {Name: "Zencoder", ParentDir: ".zencoder"},
}

// GetToolConfig returns the configuration for a tool by name.
func GetToolConfig(toolName string) (ToolConfig, bool) {
	config, ok := knownTools[toolName]
	return config, ok
}

// GetScopedSkillsDir returns the skills directory for a tool at a given scope.
// For user scope, it returns the tool's existing SkillsDir.
// For project scope, it computes the path relative to the repo root.
func GetScopedSkillsDir(tool Tool, scope Scope, repoRoot string) string {
	if scope == ScopeUser {
		return tool.SkillsDir
	}

	// For project scope, compute the path from the tool name
	config, ok := GetToolConfig(tool.Name)
	if !ok {
		// Fallback: extract parent dir from the tool's skills dir
		// e.g., ~/.claude/skills -> .claude
		parentDir := extractParentDir(tool.SkillsDir)
		return filepath.Join(repoRoot, parentDir, "skills")
	}

	return filepath.Join(repoRoot, config.ParentDir, "skills")
}

// extractParentDir extracts the tool's parent directory from its skills path.
// For example, ~/.claude/skills -> .claude
func extractParentDir(skillsDir string) string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return ".claude" // default fallback
	}

	// Remove home prefix
	rel := strings.TrimPrefix(skillsDir, home)
	rel = strings.TrimPrefix(rel, "/")

	// Remove "/skills" suffix
	rel = strings.TrimSuffix(rel, "/skills")
	rel = strings.TrimSuffix(rel, "skills")

	if rel == "" {
		return ".claude" // default fallback
	}

	return rel
}

// GetProjectToolsForScope returns tools configured for project scope installation.
// Unlike DetectTools which checks if the tool is installed globally,
// this returns all known tools so users can install to project scope
// even if the tool isn't installed on their machine.
func GetProjectToolsForScope() []Tool {
	var tools []Tool
	for name, config := range knownTools {
		tools = append(tools, Tool{
			Name:      name,
			SkillsDir: filepath.Join(config.ParentDir, "skills"), // relative path
		})
	}
	return tools
}

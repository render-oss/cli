package skills

import (
	"testing"
)

func TestScopeIsValid(t *testing.T) {
	tests := []struct {
		scope Scope
		want  bool
	}{
		{ScopeUser, true},
		{ScopeProject, true},
		{Scope("invalid"), false},
		{Scope(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.scope), func(t *testing.T) {
			if got := tt.scope.IsValid(); got != tt.want {
				t.Errorf("Scope(%q).IsValid() = %v, want %v", tt.scope, got, tt.want)
			}
		})
	}
}

func TestParseScope(t *testing.T) {
	tests := []struct {
		input   string
		want    Scope
		wantErr bool
	}{
		{"user", ScopeUser, false},
		{"USER", ScopeUser, false},
		{"User", ScopeUser, false},
		{"project", ScopeProject, false},
		{"PROJECT", ScopeProject, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseScope(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseScope(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseScope(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetToolConfig(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		wantOK   bool
	}{
		{"Claude Code exists", "Claude Code", true},
		{"Cursor exists", "Cursor", true},
		{"Unknown tool", "Unknown Tool", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := GetToolConfig(tt.toolName)
			if ok != tt.wantOK {
				t.Errorf("GetToolConfig(%q) ok = %v, want %v", tt.toolName, ok, tt.wantOK)
			}
		})
	}
}

func TestGetScopedSkillsDir(t *testing.T) {
	tool := Tool{
		Name:      "Claude Code",
		SkillsDir: "/home/user/.claude/skills",
	}
	repoRoot := "/path/to/repo"

	// User scope should return the tool's existing skills dir
	userDir := GetScopedSkillsDir(tool, ScopeUser, repoRoot)
	if userDir != tool.SkillsDir {
		t.Errorf("GetScopedSkillsDir(user) = %q, want %q", userDir, tool.SkillsDir)
	}

	// Project scope should return a path under the repo root
	projectDir := GetScopedSkillsDir(tool, ScopeProject, repoRoot)
	expected := "/path/to/repo/.claude/skills"
	if projectDir != expected {
		t.Errorf("GetScopedSkillsDir(project) = %q, want %q", projectDir, expected)
	}
}

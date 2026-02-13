package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

var defaultStatePath string

// InstalledSkill records the name, version, and content hash of an installed skill.
type InstalledSkill struct {
	Name    string `yaml:"name"`
	DirName string `yaml:"dir_name,omitempty"` // filesystem directory name
	Version string `yaml:"version"`
	Hash    string `yaml:"hash,omitempty"`
	Scope   Scope  `yaml:"scope,omitempty"` // installation scope (user or project)
}

// EffectiveScope returns the skill's scope, treating empty scope as ScopeUser.
// Legacy state files created before scope tracking default to user scope.
func (s InstalledSkill) EffectiveScope() Scope {
	if s.Scope == "" {
		return ScopeUser
	}
	return s.Scope
}

// EffectiveDirName returns the directory name for filesystem operations.
// It falls back to Name for state files created before DirName was tracked.
func (s InstalledSkill) EffectiveDirName() string {
	if s.DirName != "" {
		return s.DirName
	}
	return s.Name
}

// SkillsState holds the user's skill installation selections so they can be
// replayed by `render skills update`.
type SkillsState struct {
	Tools       []string         `yaml:"tools"`
	Skills      []InstalledSkill `yaml:"skills"`
	InstalledAt string           `yaml:"installed_at,omitempty"`
}

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fall back to a safe default; the actual operations will handle the error
		defaultStatePath = ""
		return
	}
	defaultStatePath = filepath.Join(home, ".render", "skills.yaml")
}

// LoadState reads the skills state file from ~/.render/skills.yaml.
// If the file does not exist, an empty state is returned.
func LoadState() (*SkillsState, error) {
	if defaultStatePath == "" {
		return &SkillsState{}, nil
	}
	data, err := os.ReadFile(defaultStatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &SkillsState{}, nil
		}
		return nil, err
	}

	var state SkillsState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// Save persists the skills state to ~/.render/skills.yaml.
func (s *SkillsState) Save() error {
	if defaultStatePath == "" {
		return fmt.Errorf("cannot save state: home directory not available")
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(defaultStatePath), 0o755); err != nil {
		return err
	}

	return os.WriteFile(defaultStatePath, data, 0o600)
}

// HasSelections returns true if the state contains both tool and skill selections.
func (s *SkillsState) HasSelections() bool {
	return len(s.Tools) > 0 && len(s.Skills) > 0
}

// Touch updates InstalledAt to the current time.
func (s *SkillsState) Touch() {
	s.InstalledAt = time.Now().UTC().Format(time.RFC3339)
}

package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateSaveAndLoad(t *testing.T) {
	// Override default state path to use a temp dir.
	dir := t.TempDir()
	origPath := defaultStatePath
	defaultStatePath = filepath.Join(dir, "skills.yaml")
	t.Cleanup(func() { defaultStatePath = origPath })

	state := &SkillsState{
		Tools: []string{"Claude Code", "Cursor"},
		Skills: []InstalledSkill{
			{Name: "render-deploy", Version: "1.0.0", Hash: "abc123"},
			{Name: "render-debug", Version: "2.0.0", Hash: "def456"},
		},
	}
	state.Touch()

	require.NoError(t, state.Save())

	loaded, err := LoadState()
	require.NoError(t, err)

	assert.Equal(t, state.Tools, loaded.Tools)
	assert.Equal(t, state.Skills, loaded.Skills)
	assert.NotEmpty(t, loaded.InstalledAt)
}

func TestLoadStateMissingFile(t *testing.T) {
	dir := t.TempDir()
	origPath := defaultStatePath
	defaultStatePath = filepath.Join(dir, "nonexistent", "skills.yaml")
	t.Cleanup(func() { defaultStatePath = origPath })

	state, err := LoadState()
	require.NoError(t, err)
	assert.Empty(t, state.Tools)
	assert.Empty(t, state.Skills)
}

func TestHasSelections(t *testing.T) {
	tests := []struct {
		name  string
		state SkillsState
		want  bool
	}{
		{
			name:  "empty state",
			state: SkillsState{},
			want:  false,
		},
		{
			name:  "tools only",
			state: SkillsState{Tools: []string{"Cursor"}},
			want:  false,
		},
		{
			name:  "skills only",
			state: SkillsState{Skills: []InstalledSkill{{Name: "render-deploy"}}},
			want:  false,
		},
		{
			name: "both",
			state: SkillsState{
				Tools:  []string{"Cursor"},
				Skills: []InstalledSkill{{Name: "render-deploy"}},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.state.HasSelections())
		})
	}
}

func TestTouch(t *testing.T) {
	state := &SkillsState{}
	assert.Empty(t, state.InstalledAt)

	state.Touch()
	assert.NotEmpty(t, state.InstalledAt)
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	origPath := defaultStatePath
	defaultStatePath = filepath.Join(dir, "nested", "deep", "skills.yaml")
	t.Cleanup(func() { defaultStatePath = origPath })

	state := &SkillsState{
		Tools:  []string{"Cursor"},
		Skills: []InstalledSkill{{Name: "render-deploy"}},
	}

	require.NoError(t, state.Save())

	_, err := os.Stat(defaultStatePath)
	assert.NoError(t, err)
}

package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTextareaConfigFromStruct(t *testing.T) {
	type sample struct {
		Name  string `cli:"name"`
		Input string `cli:"input" cli-lines:"15" cli-ext:"json"`
		Notes string `cli:"notes" cli-lines:"5"`
	}

	configs := textareaConfigFromStruct(&sample{})

	t.Run("field with both cli-lines and cli-ext", func(t *testing.T) {
		cfg, ok := configs["input"]
		assert.True(t, ok)
		assert.Equal(t, 15, cfg.lines)
		assert.Equal(t, "json", cfg.ext)
	})

	t.Run("field with cli-lines only", func(t *testing.T) {
		cfg, ok := configs["notes"]
		assert.True(t, ok)
		assert.Equal(t, 5, cfg.lines)
		assert.Equal(t, "", cfg.ext)
	})

	t.Run("field with no textarea tags is excluded", func(t *testing.T) {
		_, ok := configs["name"]
		assert.False(t, ok)
	})
}

func TestPreferredEditor(t *testing.T) {
	t.Run("returns $EDITOR when set", func(t *testing.T) {
		t.Setenv("EDITOR", "emacs")
		assert.Equal(t, "emacs", preferredEditor())
	})

	t.Run("returns vi when $EDITOR is unset", func(t *testing.T) {
		t.Setenv("EDITOR", "")
		assert.Equal(t, "vi", preferredEditor())
	})

	t.Run("respects nano if user explicitly set it", func(t *testing.T) {
		t.Setenv("EDITOR", "nano")
		assert.Equal(t, "nano", preferredEditor())
	})
}

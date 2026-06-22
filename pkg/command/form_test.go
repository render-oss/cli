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

func TestRequiredFieldCLITags(t *testing.T) {
	type sample struct {
		Name         *string `cli:"name" validate:"required"`
		Branch       *string `cli:"branch"`
		BuildCommand *string `cli:"build-command" validate:"required"`
	}

	required := requiredFieldCLITags(&sample{})

	assert.True(t, required["name"])
	assert.True(t, required["build-command"])
	assert.False(t, required["branch"])
}

func TestRequiredFieldError(t *testing.T) {
	assert.EqualError(t, requiredFieldError("input", false), "input is required")
	assert.EqualError(t, requiredFieldError("input", true), "--input is required")
}

func TestValidateRequiredFields(t *testing.T) {
	t.Run("missing required pointer string", func(t *testing.T) {
		input := struct {
			Name *string `cli:"name" validate:"required"`
		}{}
		err := ValidateRequiredFields(&input)
		assert.EqualError(t, err, "--name is required")
	})

	t.Run("empty required pointer string", func(t *testing.T) {
		name := ""
		input := struct {
			Name *string `cli:"name" validate:"required"`
		}{Name: &name}
		err := ValidateRequiredFields(&input)
		assert.EqualError(t, err, "--name is required")
	})

	t.Run("all required fields present", func(t *testing.T) {
		name := "my-workflow"
		repo := "https://github.com/org/repo"
		runtime := "node"
		build := "npm install"
		run := "npm start"
		input := struct {
			Name         *string `cli:"name" validate:"required"`
			Repo         *string `cli:"repo" validate:"required"`
			Runtime      *string `cli:"runtime" validate:"required"`
			BuildCommand *string `cli:"build-command" validate:"required"`
			RunCommand   *string `cli:"run-command" validate:"required"`
		}{
			Name:         &name,
			Repo:         &repo,
			Runtime:      &runtime,
			BuildCommand: &build,
			RunCommand:   &run,
		}
		assert.NoError(t, ValidateRequiredFields(&input))
	})

	t.Run("empty required slice", func(t *testing.T) {
		input := struct {
			EnvVars []string `cli:"env-var" validate:"required"`
		}{}
		err := ValidateRequiredFields(&input)
		assert.EqualError(t, err, "--env-var is required")
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

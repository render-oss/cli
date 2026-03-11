package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	service "github.com/render-oss/cli/pkg/types/service"
)

func TestParsePreviewsGeneration(t *testing.T) {
	t.Run("parses automatic", func(t *testing.T) {
		result, err := service.ParsePreviewsGeneration("automatic")
		require.NoError(t, err)
		assert.Equal(t, service.PreviewsGenerationAutomatic, result)
	})

	t.Run("parses manual", func(t *testing.T) {
		result, err := service.ParsePreviewsGeneration("manual")
		require.NoError(t, err)
		assert.Equal(t, service.PreviewsGenerationManual, result)
	})

	t.Run("parses off", func(t *testing.T) {
		result, err := service.ParsePreviewsGeneration("off")
		require.NoError(t, err)
		assert.Equal(t, service.PreviewsGenerationOff, result)
	})

	t.Run("handles leading whitespace", func(t *testing.T) {
		result, err := service.ParsePreviewsGeneration(" automatic")
		require.NoError(t, err)
		assert.Equal(t, service.PreviewsGenerationAutomatic, result)
	})

	t.Run("handles trailing whitespace", func(t *testing.T) {
		result, err := service.ParsePreviewsGeneration("manual  ")
		require.NoError(t, err)
		assert.Equal(t, service.PreviewsGenerationManual, result)
	})

	t.Run("handles leading and trailing whitespace", func(t *testing.T) {
		result, err := service.ParsePreviewsGeneration("  off  ")
		require.NoError(t, err)
		assert.Equal(t, service.PreviewsGenerationOff, result)
	})

	t.Run("rejects invalid string", func(t *testing.T) {
		_, err := service.ParsePreviewsGeneration("invalid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "previews must be one of")
	})

	t.Run("rejects empty string", func(t *testing.T) {
		_, err := service.ParsePreviewsGeneration("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "previews must be one of")
	})
}

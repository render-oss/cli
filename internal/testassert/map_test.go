package testassert_test

import (
	"testing"

	"github.com/render-oss/cli/internal/testassert"
	"github.com/stretchr/testify/assert"
)

func TestMapContains(t *testing.T) {
	actual := map[string]any{
		"data": map[string]any{
			"id":   "srv-123",
			"name": "my-api",
			"serviceDetails": map[string]any{
				"runtime":            "node",
				"envSpecificDetails": map[string]any{},
			},
		},
		"extra": "ignored",
	}

	t.Run("passes when actual contains expected keys and values", func(t *testing.T) {
		assert.True(t, testassert.MapContains(t, actual, map[string]any{
			"data": map[string]any{
				"id": "srv-123",
				"serviceDetails": map[string]any{
					"runtime": "node",
				},
			},
		}))
	})

	t.Run("missing key fails", func(t *testing.T) {
		spy := &spyT{}
		assert.False(t, testassert.MapContains(spy, actual, map[string]any{
			"data": map[string]any{
				"missing": true,
			},
		}))
		assert.True(t, spy.failed)
	})

	t.Run("wrong value fails", func(t *testing.T) {
		spy := &spyT{}
		assert.False(t, testassert.MapContains(spy, actual, map[string]any{
			"data": map[string]any{
				"id": "srv-456",
			},
		}))
		assert.True(t, spy.failed)
	})

	t.Run("wrong nested type fails", func(t *testing.T) {
		spy := &spyT{}
		assert.False(t, testassert.MapContains(spy, actual, map[string]any{
			"data": map[string]any{
				"name": map[string]any{
					"first": "my-api",
				},
			},
		}))
		assert.True(t, spy.failed)
	})
}

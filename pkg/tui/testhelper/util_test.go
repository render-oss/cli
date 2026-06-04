package testhelper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWaitForContainsOptions(t *testing.T) {
	t.Run("uses defaults for zero values", func(t *testing.T) {
		options := waitForContainsOptions(WaitForContainsOptions{})

		assert.Equal(t, 3*time.Second, options.Duration)
		assert.Equal(t, 10*time.Millisecond, options.CheckInterval)
	})

	t.Run("layers non-zero values over defaults", func(t *testing.T) {
		options := waitForContainsOptions(WaitForContainsOptions{
			Duration: 5 * time.Second,
		})

		assert.Equal(t, 5*time.Second, options.Duration)
		assert.Equal(t, 10*time.Millisecond, options.CheckInterval)
	})

	t.Run("panics when multiple options are passed", func(t *testing.T) {
		assert.Panics(t, func() {
			waitForContainsOptions(
				WaitForContainsOptions{Duration: 5 * time.Second},
				WaitForContainsOptions{CheckInterval: 5 * time.Millisecond},
			)
		})
	})
}

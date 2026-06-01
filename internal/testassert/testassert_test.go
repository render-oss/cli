package testassert_test

import (
	"testing"

	"github.com/render-oss/cli/internal/testassert"
	"github.com/stretchr/testify/assert"
)

func TestContainsInOrder(t *testing.T) {
	const s = "alpha bravo charlie"

	t.Run("substrings in order pass", func(t *testing.T) {
		assert.True(t, testassert.ContainsInOrder(t, s, "alpha", "charlie"))
	})

	t.Run("single substring present", func(t *testing.T) {
		assert.True(t, testassert.ContainsInOrder(t, s, "bravo"))
	})

	// Vacuously true with no substrings, like strings.Contains(s, "") — asserting
	// nothing can't be violated, and it keeps dynamically-built arg lists safe.
	t.Run("no substrings always passes", func(t *testing.T) {
		assert.True(t, testassert.ContainsInOrder(t, s))
	})

	t.Run("out-of-order fails", func(t *testing.T) {
		// "alpha" appears before "charlie", so requesting them reversed fails.
		spy := &spyT{}
		assert.False(t, testassert.ContainsInOrder(spy, s, "charlie", "alpha"))
		assert.True(t, spy.failed, "expected out-of-order assertion to fail")
	})

	t.Run("missing substring fails", func(t *testing.T) {
		spy := &spyT{}
		assert.False(t, testassert.ContainsInOrder(spy, s, "delta"))
		assert.True(t, spy.failed, "expected missing substring to fail")
	})

	t.Run("repeated substring needs two occurrences", func(t *testing.T) {
		// Only one "alpha", so requiring it twice must fail.
		spy := &spyT{}
		assert.False(t, testassert.ContainsInOrder(spy, s, "alpha", "alpha"))
		assert.True(t, spy.failed, "expected second occurrence requirement to fail")
	})
}

// spyT implements assert.TestingT (just Errorf) so we can observe failures on
// the negative cases without failing the real test.
type spyT struct{ failed bool }

func (s *spyT) Errorf(string, ...any) { s.failed = true }

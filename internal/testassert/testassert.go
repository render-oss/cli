// Package testassert provides additional assertion helpers that
// stretchr/testify does not cover.
package testassert

import (
	"fmt"
	"strings"

	"github.com/stretchr/testify/assert"
)

// ContainsInOrder asserts that every substr appears in s AND that each appears
// after the previous one (an ordered subsequence).
// Handy for asserting the layout of rendered text output.
// Each substr is matched at its first occurrence at or after the end
// of the previous match. Returns true when the assertion passes.
//
// With no substrings it passes, matching the convention of
// strings.Contains(s, "") and testify's assert.Subset with an empty subset.
//
// It accepts the same minimal interface as testify's own assertions (a
// *testing.T satisfies it), and reports the failing position through
// t.Helper()/t.Errorf so failures point at the caller.
func ContainsInOrder(t assert.TestingT, s string, substrs ...string) bool {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	pos := 0
	for _, sub := range substrs {
		i := strings.Index(s[pos:], sub)
		if i < 0 {
			return assert.Fail(t, fmt.Sprintf("substring %q not found in expected order", sub),
				fmt.Sprintf("expected it after offset %d in:\n%s", pos, s))
		}
		pos += i + len(sub)
	}
	return true
}

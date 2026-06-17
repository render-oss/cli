package testassert

import (
	"fmt"
	"strings"

	"github.com/stretchr/testify/assert"
)

// MapContains asserts that every key and value in expected is present in
// actual. Extra keys in actual are ignored. Nested map[string]any values are
// compared recursively.
func MapContains(t assert.TestingT, actual, expected map[string]any) bool {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	return mapContains(t, actual, expected, nil)
}

func mapContains(t assert.TestingT, actual, expected map[string]any, path []string) bool {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	ok := true
	for key, expectedValue := range expected {
		keyPath := append(path, key)
		actualValue, exists := actual[key]
		if !exists {
			ok = assert.Fail(t, fmt.Sprintf("expected map to contain key %q", strings.Join(keyPath, "."))) && ok
			continue
		}

		expectedMap, expectedIsMap := expectedValue.(map[string]any)
		if expectedIsMap {
			actualMap, actualIsMap := actualValue.(map[string]any)
			if !actualIsMap {
				ok = assert.Fail(t,
					fmt.Sprintf("expected %q to contain a map", strings.Join(keyPath, ".")),
					fmt.Sprintf("actual value: %#v", actualValue),
				) && ok
				continue
			}
			ok = mapContains(t, actualMap, expectedMap, keyPath) && ok
			continue
		}

		ok = assert.Equal(t, expectedValue, actualValue, "expected %q value", strings.Join(keyPath, ".")) && ok
	}
	return ok
}

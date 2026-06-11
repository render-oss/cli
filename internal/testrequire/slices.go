package testrequire

import "github.com/stretchr/testify/require"

// SubSlice returns the nested slice stored at key, failing the test if the key
// is absent or the value is not a []any.
func SubSlice(t require.TestingT, body map[string]any, key string) []any {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	require.Contains(t, body, key, "expected %q", key)
	require.IsType(t, []any{}, body[key], "expected %q to contain a slice", key)
	return body[key].([]any)
}

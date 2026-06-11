package testrequire

import "github.com/stretchr/testify/require"

// SubMap returns the nested map stored at key, failing the test if the key is
// absent or the value is not a map[string]any.
func SubMap(t require.TestingT, body map[string]any, key string) map[string]any {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	require.Contains(t, body, key, "expected %q", key)
	require.IsType(t, map[string]any{}, body[key], "expected %q to contain a map", key)
	return body[key].(map[string]any)
}

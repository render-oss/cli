package testrequire

import (
	"encoding/json"

	"github.com/stretchr/testify/require"
)

// AsJSONMap marshals v as JSON and unmarshals it into a map[string]any,
// failing the test if either step fails.
// Use this, for instance, to assert the serialized form of a struct.
func AsJSONMap(t require.TestingT, v any) map[string]any {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	encoded, err := json.Marshal(v)
	require.NoError(t, err, "Serializes to JSON")

	return parseJSONMap(t, encoded, "Round-trips back to a map")
}

// ParseJSONMap unmarshals string-encoded JSON into a map[string]any, failing
// the test if the JSON is invalid or does not contain an object.
func ParseJSONMap(t require.TestingT, raw string) map[string]any {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	return parseJSONMap(t, []byte(raw), "Parses JSON string into a map")
}

func parseJSONMap(t require.TestingT, encoded []byte, msg string) map[string]any {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	var body map[string]any
	require.NoError(t, json.Unmarshal(encoded, &body), msg)
	return body
}

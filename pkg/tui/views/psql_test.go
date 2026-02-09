package views

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPSQLResult_JSONMarshal(t *testing.T) {
	tests := []struct {
		name     string
		result   PSQLResult
		expected string
	}{
		{
			name:     "tabular output",
			result:   PSQLResult{Output: " id | name\n----+------\n  1 | test\n(1 row)\n"},
			expected: `{"output":" id | name\n----+------\n  1 | test\n(1 row)\n"}`,
		},
		{
			name:     "empty output",
			result:   PSQLResult{Output: ""},
			expected: `{"output":""}`,
		},
		{
			name:     "csv output from passthrough",
			result:   PSQLResult{Output: "id,name\n1,alice\n2,bob\n"},
			expected: `{"output":"id,name\n1,alice\n2,bob\n"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.result)
			require.NoError(t, err)
			require.JSONEq(t, tt.expected, string(b))
		})
	}
}

func TestPSQLResult_YAMLMarshal(t *testing.T) {
	result := PSQLResult{Output: "hello world\n"}

	b, err := yaml.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(b), "output:")
	require.Contains(t, string(b), "hello world")
}

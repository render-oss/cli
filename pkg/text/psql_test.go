package text

import (
	"testing"

	"github.com/render-oss/cli/pkg/tui/views"
	"github.com/stretchr/testify/require"
)

func TestPSQLResultText(t *testing.T) {
	tests := []struct {
		name     string
		result   *views.PSQLResult
		expected string
	}{
		{
			name:     "returns raw output",
			result:   &views.PSQLResult{Output: " id | name\n----+------\n  1 | test\n(1 row)\n"},
			expected: " id | name\n----+------\n  1 | test\n(1 row)\n",
		},
		{
			name:     "empty output",
			result:   &views.PSQLResult{Output: ""},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PSQLResultText(tt.result)
			require.Equal(t, tt.expected, result)
		})
	}
}

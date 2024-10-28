package command_test

import (
	"testing"
	"time"

	"github.com/renderinc/render-cli/pkg/command"
	"github.com/stretchr/testify/require"
)

func TestParseTime(t *testing.T) {
	now := time.Now().Truncate(time.Second).UTC()
	tcs := []struct {
		name     string
		str      string
		expected time.Time
	}{
		{
			name:     "parse relative time minute",
			str:      "1m",
			expected: now.Add(-time.Minute),
		},
		{
			name:     "parse relative time hour",
			str:      "1h",
			expected: now.Add(-time.Hour),
		},
		{
			name:     "parse relative time day",
			str:      "1d",
			expected: now.Add(-24 * time.Hour),
		},
		{
			name:     "parse absolute time",
			str:      now.Format(time.RFC3339),
			expected: now,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			actual := command.ParseTime(now, &tc.str)
			require.Equal(t, tc.expected, *actual)
		})
	}
}

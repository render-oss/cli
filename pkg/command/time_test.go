package command_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/pointers"
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
			actual, err := command.ParseTime(now, &tc.str)
			require.NoError(t, err)
			require.Equal(t, tc.expected, *actual)
		})
	}

	t.Run("should handle nil", func(t *testing.T) {
		actual, err := command.ParseTime(now, nil)
		require.NoError(t, err)
		require.Nil(t, actual)
	})

	t.Run("should not error when passed empty string", func(t *testing.T) {
		actual, err := command.ParseTime(now, pointers.From(""))
		require.NoError(t, err)
		require.Nil(t, actual)
	})

	t.Run("should error when passed time is invalid", func(t *testing.T) {
		_, err := command.ParseTime(now, pointers.From("a long time ago"))
		require.Error(t, err)
	})
}

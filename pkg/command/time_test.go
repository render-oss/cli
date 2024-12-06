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
		expected command.TimeOrRelative
	}{
		{
			name:     "parse relative time minute",
			str:      "1m",
			expected: command.TimeOrRelative{T: pointers.From(now.Add(-time.Minute)), Relative: pointers.From("1m")},
		},
		{
			name:     "parse relative time hour",
			str:      "1h",
			expected: command.TimeOrRelative{T: pointers.From(now.Add(-time.Hour)), Relative: pointers.From("1h")},
		},
		{
			name:     "parse relative time day",
			str:      "1d",
			expected: command.TimeOrRelative{T: pointers.From(now.Add(-24 * time.Hour)), Relative: pointers.From("1d")},
		},
		{
			name:     "parse absolute time",
			str:      now.Format(time.RFC3339),
			expected: command.TimeOrRelative{T: &now},
		},
		{
			name: "trims whitespace",
			str:  "  1m  ",
			expected: command.TimeOrRelative{
				T:        pointers.From(now.Add(-time.Minute)),
				Relative: pointers.From("1m"),
			},
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

func TestCobraTime(t *testing.T) {
	t.Run("set to relative time", func(t *testing.T) {
		cobraTime := command.CobraTime{}
		err := cobraTime.Set("1m")
		require.NoError(t, err)

		require.Equal(t, "1m", cobraTime.String())
		require.WithinDuration(t, time.Now().Add(-time.Minute), *cobraTime.Get().T, time.Second)
	})

	t.Run("set to absolute time", func(t *testing.T) {
		now := time.Now().Truncate(time.Second).UTC()
		cobraTime := command.CobraTime{}
		err := cobraTime.Set(now.Format(time.RFC3339))
		require.NoError(t, err)

		require.Equal(t, now.Format(time.RFC3339), cobraTime.String())
		require.Equal(t, now, *cobraTime.Get().T)
	})
}

func TestTimeSuggestion(t *testing.T) {
	tcs := []struct {
		name     string
		str      string
		expected []string
	}{
		{
			name:     "empty string",
			str:      "",
			expected: []string{"2006-01-02T15:04:05Z"},
		},
		{
			name:     "< 60 int",
			str:      "20",
			expected: []string{"20m"},
		},
		{
			name:     "match time format",
			str:      "202",
			expected: []string{"2026-01-02T15:04:05Z"},
		},
		{
			name:     "no suggestion if time is too long",
			str:      "2026-01-02T15:04:05ZABC",
			expected: []string{""},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			actual := command.TimeSuggestion(tc.str)
			require.ElementsMatch(t, tc.expected, actual)
		})
	}
}

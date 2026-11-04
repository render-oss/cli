package views_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/tui/views"
)

func TestRunLogInput(t *testing.T) {
	startedAt := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(time.Minute)

	t.Run("finished run shows its window", func(t *testing.T) {
		input := views.RunLogInput("wfl-1", "frn-1", &startedAt, &completedAt, true)

		assert.Equal(t, []string{"wfl-1"}, input.ResourceIDs)
		assert.Equal(t, []string{"frn-1"}, input.TaskRunID)
		require.NotNil(t, input.StartTime)
		assert.Equal(t, startedAt.Add(-5*time.Second), *input.StartTime.T)
		require.NotNil(t, input.EndTime)
		assert.Equal(t, completedAt.Add(5*time.Second), *input.EndTime.T)
		assert.False(t, input.Tail)
	})

	t.Run("unfinished run tails from its start", func(t *testing.T) {
		input := views.RunLogInput("wfl-1", "trn-1", &startedAt, nil, false)

		require.NotNil(t, input.StartTime)
		assert.Equal(t, startedAt.Add(-5*time.Second), *input.StartTime.T)
		assert.Nil(t, input.EndTime)
		assert.True(t, input.Tail)
	})

	t.Run("pending run has no window", func(t *testing.T) {
		input := views.RunLogInput("wfl-1", "trn-1", nil, nil, false)

		assert.Nil(t, input.StartTime)
		assert.Nil(t, input.EndTime)
		assert.True(t, input.Tail)
	})
}

func TestStartedRunLogInput(t *testing.T) {
	before := time.Now()
	input := views.StartedRunLogInput("wfl-1", "frn-1")

	assert.Equal(t, []string{"wfl-1"}, input.ResourceIDs)
	assert.Equal(t, []string{"frn-1"}, input.TaskRunID)
	assert.True(t, input.Tail)
	assert.Nil(t, input.EndTime)
	require.NotNil(t, input.StartTime)
	assert.WithinDuration(t, before.Add(-10*time.Second), *input.StartTime.T, time.Second)
}

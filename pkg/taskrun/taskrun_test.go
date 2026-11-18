package taskrun

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/evertras/bubble-table/table"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/style"
)

func TestTaskRunDetailsFormat(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		result := TaskRunDetailsFormat(nil)
		assert.Nil(t, result)
	})

	t.Run("all fields populated", func(t *testing.T) {
		startedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
		completedAt := time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC)

		var input wfclient.TaskData
		require.NoError(t, input.FromTaskData1(map[string]any{"key": "value"}))

		taskRun := &wfclient.TaskRunDetails{
			Id:          "tr-1",
			Status:      wfclient.Completed,
			StartedAt:   &startedAt,
			CompletedAt: &completedAt,
			Input:       input,
			Error:       pointers.From("something went wrong"),
			Results:     wfclient.TaskRunResult{42},
		}

		kvs := TaskRunDetailsFormat(taskRun)
		require.NotNil(t, kvs)

		kvMap := make(map[string]string, len(kvs))
		for _, kv := range kvs {
			kvMap[kv.Key] = kv.Value
		}

		assert.Equal(t, "tr-1", kvMap["ID"])
		assert.Equal(t, "Succeeded", ansi.Strip(kvMap["Status"]))
		assert.Equal(t, startedAt.Format(time.RFC3339), kvMap["Started At"])
		assert.Equal(t, completedAt.Format(time.RFC3339), kvMap["Completed At"])
		assert.Contains(t, kvMap["Input"], "key")
		assert.Equal(t, "something went wrong", kvMap["Error"])
		assert.Contains(t, kvMap["Results"], "42")
	})

	t.Run("nil StartedAt and CompletedAt produce empty string values", func(t *testing.T) {
		var input wfclient.TaskData
		require.NoError(t, input.FromTaskData0([]any{}))

		taskRun := &wfclient.TaskRunDetails{
			Id:          "tr-2",
			Status:      wfclient.Pending,
			StartedAt:   nil,
			CompletedAt: nil,
			Input:       input,
		}

		kvs := TaskRunDetailsFormat(taskRun)
		kvMap := make(map[string]string, len(kvs))
		for _, kv := range kvs {
			kvMap[kv.Key] = kv.Value
		}

		assert.Equal(t, "", kvMap["Started At"])
		assert.Equal(t, "", kvMap["Completed At"])
	})

	t.Run("error field appended when non-nil", func(t *testing.T) {
		var input wfclient.TaskData
		require.NoError(t, input.FromTaskData0([]any{}))

		taskRun := &wfclient.TaskRunDetails{
			Id:     "tr-3",
			Status: wfclient.Failed,
			Input:  input,
			Error:  pointers.From("task timed out"),
		}

		kvs := TaskRunDetailsFormat(taskRun)
		keys := make([]string, 0, len(kvs))
		for _, kv := range kvs {
			keys = append(keys, kv.Key)
		}

		assert.Contains(t, keys, "Error")
	})

	t.Run("error field omitted when nil", func(t *testing.T) {
		var input wfclient.TaskData
		require.NoError(t, input.FromTaskData0([]any{}))

		taskRun := &wfclient.TaskRunDetails{
			Id:     "tr-4",
			Status: wfclient.Running,
			Input:  input,
			Error:  nil,
		}

		kvs := TaskRunDetailsFormat(taskRun)
		keys := make([]string, 0, len(kvs))
		for _, kv := range kvs {
			keys = append(keys, kv.Key)
		}

		assert.NotContains(t, keys, "Error")
	})

	t.Run("results field appended when non-nil", func(t *testing.T) {
		var input wfclient.TaskData
		require.NoError(t, input.FromTaskData0([]any{}))

		taskRun := &wfclient.TaskRunDetails{
			Id:      "tr-5",
			Status:  wfclient.Completed,
			Input:   input,
			Results: wfclient.TaskRunResult{"result-data"},
		}

		kvs := TaskRunDetailsFormat(taskRun)
		keys := make([]string, 0, len(kvs))
		for _, kv := range kvs {
			keys = append(keys, kv.Key)
		}

		assert.Contains(t, keys, "Results")
	})

	t.Run("results field omitted when nil", func(t *testing.T) {
		var input wfclient.TaskData
		require.NoError(t, input.FromTaskData0([]any{}))

		taskRun := &wfclient.TaskRunDetails{
			Id:      "tr-6",
			Status:  wfclient.Running,
			Input:   input,
			Results: nil,
		}

		kvs := TaskRunDetailsFormat(taskRun)
		keys := make([]string, 0, len(kvs))
		for _, kv := range kvs {
			keys = append(keys, kv.Key)
		}

		assert.NotContains(t, keys, "Results")
	})
}

func TestRow(t *testing.T) {
	t.Run("pending run with no dates", func(t *testing.T) {
		taskRun := &wfclient.TaskRun{
			Id:          "tr-1",
			Status:      wfclient.Pending,
			StartedAt:   nil,
			CompletedAt: nil,
		}

		row := Row(taskRun)
		require.Len(t, row, 5)
		assert.Equal(t, "tr-1", row[0])
		assert.Equal(t, "pending", row[1])
		assert.Equal(t, "", row[2]) // started
		assert.Equal(t, "", row[3]) // completed
		assert.Equal(t, "", row[4]) // duration
	})

	t.Run("running with only StartedAt", func(t *testing.T) {
		startedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

		taskRun := &wfclient.TaskRun{
			Id:          "tr-2",
			Status:      wfclient.Running,
			StartedAt:   &startedAt,
			CompletedAt: nil,
		}

		row := Row(taskRun)
		require.Len(t, row, 5)
		assert.Equal(t, "tr-2", row[0])
		assert.Equal(t, "running", row[1])
		assert.Equal(t, startedAt.Format(time.RFC3339), row[2])
		assert.Equal(t, "", row[3]) // completed
		assert.Equal(t, "", row[4]) // duration
	})

	t.Run("completed with both dates", func(t *testing.T) {
		startedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
		completedAt := time.Date(2025, 1, 15, 10, 35, 30, 0, time.UTC)

		taskRun := &wfclient.TaskRun{
			Id:          "tr-3",
			Status:      wfclient.Completed,
			StartedAt:   &startedAt,
			CompletedAt: &completedAt,
		}

		row := Row(taskRun)
		require.Len(t, row, 5)
		assert.Equal(t, "tr-3", row[0])
		assert.Equal(t, "completed", row[1])
		assert.Equal(t, startedAt.Format(time.RFC3339), row[2])
		assert.Equal(t, completedAt.Format(time.RFC3339), row[3])

		// Duration should be 5m30s
		duration := completedAt.Sub(startedAt)
		assert.Equal(t, duration.String(), row[4])
		assert.True(t, strings.Contains(row[4], "5m30s"), "expected duration to contain 5m30s, got %s", row[4])
	})
}

func TestIsFinished(t *testing.T) {
	tests := []struct {
		status wfclient.TaskRunStatus
		want   bool
	}{
		{wfclient.Pending, false},
		{wfclient.Running, false},
		{wfclient.Paused, false},
		{wfclient.Completed, true},
		{wfclient.Succeeded, true},
		{wfclient.Failed, true},
		// Canceled runs used to be treated as unfinished, so their logs tailed forever.
		{wfclient.Canceled, true},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			assert.Equal(t, tc.want, IsFinished(tc.status))
		})
	}
}

func TestRunStatusPresentation(t *testing.T) {
	for _, tc := range []struct {
		raw         string
		label       string
		statusStyle lipgloss.Style
	}{
		{"pending", "Queued", style.Status.Foreground(style.ColorDeprioritized)},
		{"running", "In Progress", style.Status.Foreground(style.ColorInfo)},
		{"sleeping", "Sleeping", style.Status.Foreground(style.ColorWarningDeprioritized)},
		{"succeeded", "Succeeded", style.Status.Foreground(style.ColorOK)},
		{"completed", "Succeeded", style.Status.Foreground(style.ColorOK)},
		{"failed", "Failed", style.Status.Foreground(style.ColorError)},
		{"canceled", "Canceled", style.Status.Foreground(style.ColorDeprioritized)},
		{"paused", "In Progress", style.Status.Foreground(style.ColorInfo)},
		{"future_state", "future_state", style.Status.Foreground(style.ColorDeprioritized)},
		{"suspended", "suspended", style.Status.Foreground(style.ColorDeprioritized)},
		{"", "", style.Status.Foreground(style.ColorDeprioritized)},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			run := &wfclient.TaskRun{Id: "run-1", Status: wfclient.TaskRunStatus(tc.raw)}
			original := *run
			row := TableRow(run)
			cell, ok := row.Data["Status"].(table.StyledCell)
			require.True(t, ok)
			assert.Equal(t, tc.label, fmt.Sprintf("%v", cell.Data))
			assert.Equal(t, tc.statusStyle.GetForeground(), cell.Style.GetForeground())
			assert.Same(t, run, row.Data["taskRun"])
			assert.Equal(t, original, *run)
			details := &wfclient.TaskRunDetails{Id: run.Id, Status: run.Status}
			originalDetails := *details
			var status string
			for _, kv := range TaskRunDetailsFormat(details) {
				if kv.Key == "Status" {
					status = ansi.Strip(kv.Value)
				}
			}
			assert.Equal(t, tc.label, status)
			assert.Equal(t, originalDetails, *details)
		})
	}
}

func TestTaskRunTableFilterMatchesStatusLabelAndRawStatus(t *testing.T) {
	for _, tc := range []struct {
		status  wfclient.TaskRunStatus
		filters []string
	}{
		{wfclient.Pending, []string{"queued", "pending"}},
		{wfclient.Running, []string{"progress", "running"}},
		{wfclient.Paused, []string{"progress", "paused"}},
	} {
		for _, filter := range tc.filters {
			t.Run(string(tc.status)+"/"+filter, func(t *testing.T) {
				model := table.New(Columns()).
					WithRows([]table.Row{
						TableRow(&wfclient.TaskRun{Id: "matching", Status: tc.status}),
						TableRow(&wfclient.TaskRun{Id: "nonmatching", Status: wfclient.Failed}),
					}).
					Filtered(true).
					WithFilterInputValue(filter)
				rows := model.GetVisibleRows()
				require.Len(t, rows, 1)
				assert.Equal(t, "matching", rows[0].Data["ID"])
			})
		}
	}
}

func TestTaskRunTableRendersOnlyStatusLabel(t *testing.T) {
	model := table.New(Columns()).
		WithRows([]table.Row{TableRow(&wfclient.TaskRun{Id: "run-1", Status: wfclient.Pending})}).
		WithTargetWidth(100)
	view := ansi.Strip(model.View())
	assert.Contains(t, view, "Queued")
	assert.NotContains(t, view, "pending")
}

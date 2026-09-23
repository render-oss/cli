package task

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
)

func TestHeader(t *testing.T) {
	assert.Equal(t, []string{"Name", "ID", "Workflow ID", "Version ID", "Created"}, Header())
}

func TestRow(t *testing.T) {
	createdAt := time.Date(2026, time.March, 9, 15, 4, 5, 0, time.UTC)

	for _, tc := range []struct {
		name     string
		task     *wfclient.Task
		expected []string
	}{
		{
			name: "task with workflow and version IDs",
			task: &wfclient.Task{
				Name:              "send-email",
				Id:                "tsk-aaa111",
				WorkflowId:        pointers.From("wf-aaa111"),
				WorkflowVersionId: pointers.From("wfv-aaa111"),
				CreatedAt:         createdAt,
			},
			expected: []string{"send-email", "tsk-aaa111", "wf-aaa111", "wfv-aaa111", "2026-03-09T15:04:05Z"},
		},
		{
			name: "same task name from a different workflow",
			task: &wfclient.Task{
				Name:              "send-email",
				Id:                "tsk-bbb222",
				WorkflowId:        pointers.From("wf-bbb222"),
				WorkflowVersionId: pointers.From("wfv-bbb222"),
				CreatedAt:         createdAt,
			},
			expected: []string{"send-email", "tsk-bbb222", "wf-bbb222", "wfv-bbb222", "2026-03-09T15:04:05Z"},
		},
		{
			name: "missing workflow ID",
			task: &wfclient.Task{
				Name:              "send-email",
				Id:                "tsk-ccc333",
				WorkflowVersionId: pointers.From("wfv-ccc333"),
				CreatedAt:         createdAt,
			},
			expected: []string{"send-email", "tsk-ccc333", "", "wfv-ccc333", "2026-03-09T15:04:05Z"},
		},
		{
			name: "missing workflow version ID",
			task: &wfclient.Task{
				Name:       "send-email",
				Id:         "tsk-ddd444",
				WorkflowId: pointers.From("wf-ddd444"),
				CreatedAt:  createdAt,
			},
			expected: []string{"send-email", "tsk-ddd444", "wf-ddd444", "", "2026-03-09T15:04:05Z"},
		},
		{
			name: "missing both IDs",
			task: &wfclient.Task{
				Name:      "send-email",
				Id:        "tsk-eee555",
				CreatedAt: createdAt,
			},
			expected: []string{"send-email", "tsk-eee555", "", "", "2026-03-09T15:04:05Z"},
		},
		{
			name: "non-UTC created timestamp keeps RFC3339 formatting",
			task: &wfclient.Task{
				Name:              "send-email",
				Id:                "tsk-fff666",
				WorkflowId:        pointers.From("wf-fff666"),
				WorkflowVersionId: pointers.From("wfv-fff666"),
				CreatedAt:         createdAt.In(time.FixedZone("UTC-5", -5*60*60)),
			},
			expected: []string{"send-email", "tsk-fff666", "wf-fff666", "wfv-fff666", "2026-03-09T10:04:05-05:00"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := Row(tc.task)

			require.Len(t, row, len(Header()), "row must align with header")
			assert.Equal(t, tc.expected, row)
		})
	}
}

func TestRowDistinguishesSameNamedTasksAcrossWorkflows(t *testing.T) {
	createdAt := time.Date(2026, time.March, 9, 15, 4, 5, 0, time.UTC)

	first := Row(&wfclient.Task{
		Name:              "send-email",
		Id:                "tsk-aaa111",
		WorkflowId:        pointers.From("wf-aaa111"),
		WorkflowVersionId: pointers.From("wfv-aaa111"),
		CreatedAt:         createdAt,
	})
	second := Row(&wfclient.Task{
		Name:              "send-email",
		Id:                "tsk-bbb222",
		WorkflowId:        pointers.From("wf-bbb222"),
		WorkflowVersionId: pointers.From("wfv-bbb222"),
		CreatedAt:         createdAt,
	})

	header := Header()
	workflowIdx := indexOf(t, header, "Workflow ID")
	versionIdx := indexOf(t, header, "Version ID")

	assert.NotEqual(t, first[workflowIdx], second[workflowIdx])
	assert.NotEqual(t, first[versionIdx], second[versionIdx])
}

func indexOf(t *testing.T, values []string, target string) int {
	t.Helper()

	for i, v := range values {
		if v == target {
			return i
		}
	}

	require.Failf(t, "column not found", "expected header to contain %q, got %v", target, values)
	return -1
}

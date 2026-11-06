package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/tasks"
	"github.com/render-oss/cli/pkg/version"
)

// mockTaskRepo implements taskRepo for testing.
type mockTaskRepo struct {
	listTasksFn func(ctx context.Context, params *client.ListTasksParams) (client.Cursor, []*wfclient.Task, error)
	runTaskFn   func(context.Context, string, *wfclient.TaskData) (*wfclient.TaskRun, error)
}

func (m *mockTaskRepo) RunTask(ctx context.Context, slug string, input *wfclient.TaskData) (*wfclient.TaskRun, error) {
	if m.runTaskFn != nil {
		return m.runTaskFn(ctx, slug, input)
	}
	return nil, nil
}
func (m *mockTaskRepo) GetTask(context.Context, string) (*wfclient.Task, error) { return nil, nil }
func (m *mockTaskRepo) ListTasks(ctx context.Context, params *client.ListTasksParams) (client.Cursor, []*wfclient.Task, error) {
	return m.listTasksFn(ctx, params)
}
func (m *mockTaskRepo) ListTaskRuns(context.Context, *client.ListTaskRunsParams) (client.Cursor, []*wfclient.TaskRun, error) {
	return "", nil, nil
}
func (m *mockTaskRepo) GetTaskRunDetails(context.Context, string) (*wfclient.TaskRunDetails, error) {
	return nil, nil
}
func (m *mockTaskRepo) CancelTaskRun(context.Context, string) error { return nil }

// mockVersionRepo implements versionRepo for testing.
type mockVersionRepo struct {
	listVersionsFn   func(ctx context.Context, workflowID string, params *client.ListWorkflowVersionsParams) (client.Cursor, []*wfclient.WorkflowVersion, error)
	getVersionFn     func(ctx context.Context, id string) (*wfclient.WorkflowVersion, error)
	triggerReleaseFn func(ctx context.Context, workflowID string, input version.TriggerReleaseInput) error
}

func (m *mockVersionRepo) ListVersions(ctx context.Context, workflowID string, params *client.ListWorkflowVersionsParams) (client.Cursor, []*wfclient.WorkflowVersion, error) {
	if m.listVersionsFn != nil {
		return m.listVersionsFn(ctx, workflowID, params)
	}
	return "", nil, nil
}

func (m *mockVersionRepo) GetVersion(ctx context.Context, id string) (*wfclient.WorkflowVersion, error) {
	if m.getVersionFn != nil {
		return m.getVersionFn(ctx, id)
	}
	return nil, nil
}

func (m *mockVersionRepo) TriggerRelease(ctx context.Context, workflowID string, input version.TriggerReleaseInput) error {
	if m.triggerReleaseFn != nil {
		return m.triggerReleaseFn(ctx, workflowID, input)
	}
	return nil
}

// mockWorkflowRepo implements workflowRepoIface for testing. ListAllWorkflows
// returns every workflow whose ID matches the filter and records each filter.
type mockWorkflowRepo struct {
	workflows  []*wfclient.Workflow
	err        error
	listParams []*client.ListWorkflowsParams
}

func (m *mockWorkflowRepo) GetWorkflow(context.Context, string) (*wfclient.Workflow, error) {
	return nil, nil
}

func (m *mockWorkflowRepo) ListAllWorkflows(_ context.Context, params *client.ListWorkflowsParams) ([]*wfclient.Workflow, error) {
	m.listParams = append(m.listParams, params)
	if m.err != nil {
		return nil, m.err
	}
	if params.WorkflowID == nil {
		return m.workflows, nil
	}
	var matched []*wfclient.Workflow
	for _, wf := range m.workflows {
		if slices.Contains(*params.WorkflowID, wf.Id) {
			matched = append(matched, wf)
		}
	}
	return matched, nil
}

func TestCreateTaskRunErrorGuidance(t *testing.T) {
	apiErr := errors.New("received response code 404: not found")
	notFound := &tasks.NotFoundError{Task: "missing/my-task", Err: apiErr}
	for _, tc := range []struct {
		name string
		err  error
		hint bool
	}{
		{name: "not found", err: notFound, hint: true},
		{name: "wrapped not found", err: fmt.Errorf("start failed: %w", notFound), hint: true},
		{name: "local name or task ID", err: apiErr},
		{name: "forbidden", err: errors.New("received response code 403: forbidden")},
		{name: "success"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expectedRun := &wfclient.TaskRun{Id: "trn-created"}
			loader := &WorkflowLoader{taskRepo: &mockTaskRepo{
				runTaskFn: func(_ context.Context, slug string, input *wfclient.TaskData) (*wfclient.TaskRun, error) {
					require.Equal(t, "missing/my-task", slug)
					require.NotNil(t, input)
					if tc.err != nil {
						return nil, tc.err
					}
					return expectedRun, nil
				},
			}}
			run, err := loader.CreateTaskRun(context.Background(), TaskRunInput{TaskSlug: "missing/my-task", Input: "[]"})
			if tc.hint {
				require.Nil(t, run)
				require.EqualError(t, err, tc.err.Error()+". Identify your task with `workflow-slug/task-name`. Run `render workflows tasks list <workflowVersionID>` to see your tasks")
				require.ErrorIs(t, err, apiErr)
			} else if tc.err != nil {
				require.Nil(t, run)
				require.Same(t, tc.err, err)
			} else {
				require.NoError(t, err)
				require.Same(t, expectedRun, run)
			}
		})
	}
}

func TestUnmarshalInputData(t *testing.T) {
	t.Run("empty input returns error", func(t *testing.T) {
		_, err := unmarshalInputData("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Task input is required")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		_, err := unmarshalInputData("{not json}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON")
	})

	t.Run("valid JSON array parses as TaskData0", func(t *testing.T) {
		td, err := unmarshalInputData(`["a","b"]`)
		require.NoError(t, err)
		require.NotNil(t, td)

		raw, err := td.MarshalJSON()
		require.NoError(t, err)

		var arr []any
		require.NoError(t, json.Unmarshal(raw, &arr))
		assert.Len(t, arr, 2)
	})

	t.Run("valid JSON object parses as TaskData1", func(t *testing.T) {
		td, err := unmarshalInputData(`{"key":"value"}`)
		require.NoError(t, err)
		require.NotNil(t, td)

		raw, err := td.MarshalJSON()
		require.NoError(t, err)

		var obj map[string]any
		require.NoError(t, json.Unmarshal(raw, &obj))
		assert.Equal(t, "value", obj["key"])
	})

	t.Run("JSON string scalar errors", func(t *testing.T) {
		_, err := unmarshalInputData(`"just a string"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "JSON array or object")
	})

	t.Run("JSON number scalar errors", func(t *testing.T) {
		_, err := unmarshalInputData(`42`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "JSON array or object")
	})
}

func TestLoadTaskList(t *testing.T) {
	task := &wfclient.Task{Id: "tsk-1", Name: "my-task"}

	t.Run("with WorkflowVersionID passes workflowVersionId param", func(t *testing.T) {
		var capturedParams *client.ListTasksParams
		taskRepo := &mockTaskRepo{
			listTasksFn: func(_ context.Context, params *client.ListTasksParams) (client.Cursor, []*wfclient.Task, error) {
				capturedParams = params
				return "c1", []*wfclient.Task{task}, nil
			},
		}

		loader := &WorkflowLoader{taskRepo: taskRepo}
		_, result, err := loader.LoadTaskList(context.Background(), TaskListInput{
			WorkflowVersionID: "wfv-123",
		}, "")

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "tsk-1", result[0].Id)
		require.NotNil(t, capturedParams.WorkflowVersionId)
		assert.Contains(t, *capturedParams.WorkflowVersionId, "wfv-123")
	})

	t.Run("with WorkflowID and AllVersions passes workflowId param", func(t *testing.T) {
		var capturedParams *client.ListTasksParams
		taskRepo := &mockTaskRepo{
			listTasksFn: func(_ context.Context, params *client.ListTasksParams) (client.Cursor, []*wfclient.Task, error) {
				capturedParams = params
				return "c1", []*wfclient.Task{task}, nil
			},
		}

		loader := &WorkflowLoader{taskRepo: taskRepo}
		_, result, err := loader.LoadTaskList(context.Background(), TaskListInput{
			WorkflowID:  "wf-1",
			AllVersions: true,
		}, "")

		require.NoError(t, err)
		require.Len(t, result, 1)
		require.NotNil(t, capturedParams.WorkflowId)
		assert.Equal(t, []string{"wf-1"}, *capturedParams.WorkflowId)
		assert.Nil(t, capturedParams.WorkflowVersionId)
	})

	t.Run("AllVersions without a workflow lists every task", func(t *testing.T) {
		var capturedParams *client.ListTasksParams
		taskRepo := &mockTaskRepo{
			listTasksFn: func(_ context.Context, params *client.ListTasksParams) (client.Cursor, []*wfclient.Task, error) {
				capturedParams = params
				return "", []*wfclient.Task{task}, nil
			},
		}

		loader := &WorkflowLoader{taskRepo: taskRepo}
		_, _, err := loader.LoadTaskList(context.Background(), TaskListInput{AllVersions: true}, "")

		require.NoError(t, err)
		assert.Equal(t, &client.ListTasksParams{}, capturedParams)
	})

	t.Run("with WorkflowID filters to its default version", func(t *testing.T) {
		var capturedParams *client.ListTasksParams
		taskRepo := &mockTaskRepo{
			listTasksFn: func(_ context.Context, params *client.ListTasksParams) (client.Cursor, []*wfclient.Task, error) {
				capturedParams = params
				return "c1", []*wfclient.Task{task}, nil
			},
		}
		workflowRepo := &mockWorkflowRepo{workflows: []*wfclient.Workflow{
			{Id: "wf-1", DefaultVersionId: pointers.From("wfv-default")},
			{Id: "wf-2", DefaultVersionId: pointers.From("wfv-other")},
		}}

		loader := &WorkflowLoader{taskRepo: taskRepo, workflowRepo: workflowRepo}
		_, result, err := loader.LoadTaskList(context.Background(), TaskListInput{WorkflowID: "wf-1"}, "")

		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Len(t, workflowRepo.listParams, 1)
		assert.Equal(t, []string{"wf-1"}, *workflowRepo.listParams[0].WorkflowID)
		require.NotNil(t, capturedParams.WorkflowVersionId)
		assert.Equal(t, []string{"wfv-default"}, *capturedParams.WorkflowVersionId)
		assert.Nil(t, capturedParams.WorkflowId)
	})

	t.Run("without a workflow filters to every workflow's default version", func(t *testing.T) {
		var capturedParams *client.ListTasksParams
		taskRepo := &mockTaskRepo{
			listTasksFn: func(_ context.Context, params *client.ListTasksParams) (client.Cursor, []*wfclient.Task, error) {
				capturedParams = params
				return "", []*wfclient.Task{task}, nil
			},
		}
		workflowRepo := &mockWorkflowRepo{workflows: []*wfclient.Workflow{
			{Id: "wf-1", DefaultVersionId: pointers.From("wfv-1")},
			{Id: "wf-unreleased"},
			{Id: "wf-2", DefaultVersionId: pointers.From("wfv-2")},
		}}

		loader := &WorkflowLoader{taskRepo: taskRepo, workflowRepo: workflowRepo}
		_, _, err := loader.LoadTaskList(context.Background(), TaskListInput{}, "")

		require.NoError(t, err)
		require.Len(t, workflowRepo.listParams, 1)
		assert.Nil(t, workflowRepo.listParams[0].WorkflowID)
		require.NotNil(t, capturedParams.WorkflowVersionId)
		assert.Equal(t, []string{"wfv-1", "wfv-2"}, *capturedParams.WorkflowVersionId)
	})

	t.Run("no default version returns no tasks without listing them", func(t *testing.T) {
		taskRepo := &mockTaskRepo{
			listTasksFn: func(context.Context, *client.ListTasksParams) (client.Cursor, []*wfclient.Task, error) {
				t.Fatal("ListTasks should not be called: an empty filter would list every task")
				return "", nil, nil
			},
		}
		workflowRepo := &mockWorkflowRepo{workflows: []*wfclient.Workflow{{Id: "wf-1"}}}

		loader := &WorkflowLoader{taskRepo: taskRepo, workflowRepo: workflowRepo}
		_, result, err := loader.LoadTaskList(context.Background(), TaskListInput{WorkflowID: "wf-1"}, "")
		require.NoError(t, err)
		assert.Empty(t, result)

		all, err := loader.LoadAllTasks(context.Background(), TaskListInput{WorkflowID: "wf-1"})
		require.NoError(t, err)
		assert.NotNil(t, all)
		assert.Empty(t, all)
	})

	t.Run("Local lists every task without resolving versions", func(t *testing.T) {
		var capturedParams *client.ListTasksParams
		taskRepo := &mockTaskRepo{
			listTasksFn: func(_ context.Context, params *client.ListTasksParams) (client.Cursor, []*wfclient.Task, error) {
				capturedParams = params
				return "", []*wfclient.Task{task}, nil
			},
		}

		// The local loader has no version or workflow repo.
		loader := &WorkflowLoader{taskRepo: taskRepo}
		_, result, err := loader.LoadTaskList(context.Background(), TaskListInput{Local: true}, "")

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, &client.ListTasksParams{}, capturedParams)
	})

	t.Run("workflow lookup failure propagates error", func(t *testing.T) {
		workflowRepo := &mockWorkflowRepo{err: errors.New("boom")}

		loader := &WorkflowLoader{workflowRepo: workflowRepo}
		_, _, err := loader.LoadTaskList(context.Background(), TaskListInput{WorkflowID: "wf-1"}, "")

		require.ErrorContains(t, err, "boom")
	})
}

func TestReleaseVersion(t *testing.T) {
	readyVersion := &wfclient.WorkflowVersion{Id: "wfv-new", Name: "v2", Status: wfclient.Ready}

	t.Run("happy path triggers release and returns version", func(t *testing.T) {
		versionRepo := &mockVersionRepo{
			triggerReleaseFn: func(context.Context, string, version.TriggerReleaseInput) error {
				return nil
			},
			listVersionsFn: func(_ context.Context, _ string, _ *client.ListWorkflowVersionsParams) (client.Cursor, []*wfclient.WorkflowVersion, error) {
				return "c1", []*wfclient.WorkflowVersion{readyVersion}, nil
			},
		}

		loader := &WorkflowLoader{workflowVersionRepo: versionRepo}
		v, err := loader.ReleaseVersion(context.Background(), VersionReleaseInput{WorkflowID: "wf-1"})

		require.NoError(t, err)
		require.NotNil(t, v)
		assert.Equal(t, "wfv-new", v.Id)
	})

	t.Run("trigger failure propagates error", func(t *testing.T) {
		versionRepo := &mockVersionRepo{
			triggerReleaseFn: func(context.Context, string, version.TriggerReleaseInput) error {
				return errors.New("internal error")
			},
		}

		loader := &WorkflowLoader{workflowVersionRepo: versionRepo}
		_, err := loader.ReleaseVersion(context.Background(), VersionReleaseInput{WorkflowID: "wf-1"})

		require.Error(t, err)
	})

	t.Run("empty commit ID is cleared to nil", func(t *testing.T) {
		var capturedInput version.TriggerReleaseInput
		versionRepo := &mockVersionRepo{
			triggerReleaseFn: func(_ context.Context, _ string, input version.TriggerReleaseInput) error {
				capturedInput = input
				return nil
			},
			listVersionsFn: func(_ context.Context, _ string, _ *client.ListWorkflowVersionsParams) (client.Cursor, []*wfclient.WorkflowVersion, error) {
				return "c1", []*wfclient.WorkflowVersion{readyVersion}, nil
			},
		}

		emptyCommit := ""
		loader := &WorkflowLoader{workflowVersionRepo: versionRepo}
		_, err := loader.ReleaseVersion(context.Background(), VersionReleaseInput{
			WorkflowID: "wf-1",
			CommitID:   &emptyCommit,
		})

		require.NoError(t, err)
		assert.Nil(t, capturedInput.CommitId, "empty commit should be cleared to nil")
	})

	t.Run("non-empty commit ID is sent", func(t *testing.T) {
		var capturedInput version.TriggerReleaseInput
		versionRepo := &mockVersionRepo{
			triggerReleaseFn: func(_ context.Context, _ string, input version.TriggerReleaseInput) error {
				capturedInput = input
				return nil
			},
			listVersionsFn: func(_ context.Context, _ string, _ *client.ListWorkflowVersionsParams) (client.Cursor, []*wfclient.WorkflowVersion, error) {
				return "c1", []*wfclient.WorkflowVersion{readyVersion}, nil
			},
		}

		commitID := "abc123"
		loader := &WorkflowLoader{workflowVersionRepo: versionRepo}
		_, err := loader.ReleaseVersion(context.Background(), VersionReleaseInput{
			WorkflowID: "wf-1",
			CommitID:   &commitID,
		})

		require.NoError(t, err)
		require.NotNil(t, capturedInput.CommitId)
		assert.Equal(t, "abc123", *capturedInput.CommitId)
	})
}

func TestWaitForVersionRelease(t *testing.T) {
	t.Run("returns version immediately when available", func(t *testing.T) {
		v := &wfclient.WorkflowVersion{Id: "wfv-1", Status: wfclient.Ready}
		versionRepo := &mockVersionRepo{
			listVersionsFn: func(_ context.Context, _ string, _ *client.ListWorkflowVersionsParams) (client.Cursor, []*wfclient.WorkflowVersion, error) {
				return "c1", []*wfclient.WorkflowVersion{v}, nil
			},
		}

		loader := &WorkflowLoader{workflowVersionRepo: versionRepo}
		result, err := loader.WaitForVersionRelease(context.Background(), "wf-1")

		require.NoError(t, err)
		assert.Equal(t, "wfv-1", result.Id)
	})

	t.Run("returns error on repo failure", func(t *testing.T) {
		versionRepo := &mockVersionRepo{
			listVersionsFn: func(_ context.Context, _ string, _ *client.ListWorkflowVersionsParams) (client.Cursor, []*wfclient.WorkflowVersion, error) {
				return "", nil, errors.New("api error")
			},
		}

		loader := &WorkflowLoader{workflowVersionRepo: versionRepo}
		_, err := loader.WaitForVersionRelease(context.Background(), "wf-1")

		require.Error(t, err)
	})
}

func TestWaitForVersion(t *testing.T) {
	t.Run("returns immediately when version is complete", func(t *testing.T) {
		versionRepo := &mockVersionRepo{
			getVersionFn: func(_ context.Context, _ string) (*wfclient.WorkflowVersion, error) {
				return &wfclient.WorkflowVersion{Id: "wfv-1", Status: wfclient.Ready}, nil
			},
		}

		loader := &WorkflowLoader{workflowVersionRepo: versionRepo}
		v, err := loader.WaitForVersion(context.Background(), "wf-1", "wfv-1")

		require.NoError(t, err)
		assert.Equal(t, "wfv-1", v.Id)
		assert.Equal(t, wfclient.Ready, v.Status)
	})

	t.Run("returns failed version when build failed", func(t *testing.T) {
		versionRepo := &mockVersionRepo{
			getVersionFn: func(_ context.Context, _ string) (*wfclient.WorkflowVersion, error) {
				return &wfclient.WorkflowVersion{Id: "wfv-1", Status: wfclient.BuildFailed}, nil
			},
		}

		loader := &WorkflowLoader{workflowVersionRepo: versionRepo}
		v, err := loader.WaitForVersion(context.Background(), "wf-1", "wfv-1")

		require.NoError(t, err)
		assert.Equal(t, wfclient.BuildFailed, v.Status)
	})

	t.Run("polls until complete", func(t *testing.T) {
		var callCount int32
		versionRepo := &mockVersionRepo{
			getVersionFn: func(_ context.Context, _ string) (*wfclient.WorkflowVersion, error) {
				count := atomic.AddInt32(&callCount, 1)
				status := wfclient.Building
				if count >= 2 {
					status = wfclient.Ready
				}
				return &wfclient.WorkflowVersion{Id: "wfv-1", Status: status}, nil
			},
		}

		loader := &WorkflowLoader{workflowVersionRepo: versionRepo, versionPollInterval: pointers.From(time.Millisecond)}
		v, err := loader.WaitForVersion(context.Background(), "wf-1", "wfv-1")

		require.NoError(t, err)
		assert.Equal(t, wfclient.Ready, v.Status)
		assert.GreaterOrEqual(t, atomic.LoadInt32(&callCount), int32(2), "should have polled at least twice")
	})
}

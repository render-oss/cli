package store_test

import (
	"encoding/json"
	"testing"

	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/workflows/store"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
	"github.com/stretchr/testify/require"
)

func TestSetTasks(t *testing.T) {
	t.Run("can set initial tasks", func(t *testing.T) {
		store := store.NewTaskStore()

		store.SetTasks([]taskserver.Task{
			{
				Name: "test",
			},
			{
				Name: "test2",
			},
		})

		tasks := store.GetTasks()
		require.Equal(t, 2, len(tasks))
		require.ElementsMatch(t, []string{"test", "test2"}, []string{tasks[0].Name, tasks[1].Name})
	})

	t.Run("can replace tasks", func(t *testing.T) {
		store := store.NewTaskStore()

		store.SetTasks([]taskserver.Task{
			{
				Name: "test",
			},
		})

		store.SetTasks([]taskserver.Task{
			{
				Name: "test2",
			},
		})

		tasks := store.GetTasks()
		require.Equal(t, 1, len(tasks))
		require.Equal(t, "test2", tasks[0].Name)
	})

	t.Run("id is preserved if task already exists", func(t *testing.T) {
		store := store.NewTaskStore()

		store.SetTasks([]taskserver.Task{
			{
				Name: "test",
			},
		})
		tasks := store.GetTasks()
		originalTaskID := tasks[0].ID

		store.SetTasks([]taskserver.Task{
			{
				Name: "test",
			},
		})

		tasks = store.GetTasks()
		require.Equal(t, "test", tasks[0].Name)
		require.Equal(t, originalTaskID, tasks[0].ID)
	})
}

func TestTaskRun(t *testing.T) {
	t.Run("can start task run", func(t *testing.T) {
		s := store.NewTaskStore()

		s.SetTasks([]taskserver.Task{
			{
				Name: "test",
			},
		})

		tasks := s.GetTasks()

		s.StartTaskRun(tasks[0].Name, []byte{}, nil)

		taskRuns := s.GetTaskRuns(tasks[0].ID)
		require.Equal(t, 1, len(taskRuns))
		require.Equal(t, tasks[0].Name, taskRuns[0].TaskName)
		require.Equal(t, store.TaskRunStatusRunning, taskRuns[0].Status)
	})
	t.Run("can complete task run", func(t *testing.T) {
		s := store.NewTaskStore()

		s.SetTasks([]taskserver.Task{
			{
				Name: "test",
			},
		})

		tasks := s.GetTasks()

		s.StartTaskRun(tasks[0].Name, []byte{}, nil)

		taskRuns := s.GetTaskRuns(tasks[0].ID)
		s.CompleteTaskRun(taskRuns[0].ID, []byte("test"))

		taskRuns = s.GetTaskRuns(tasks[0].ID)
		require.Equal(t, store.TaskRunStatusComplete, taskRuns[0].Status)
		require.Equal(t, json.RawMessage("test"), taskRuns[0].Output)
	})

	t.Run("can fail task run", func(t *testing.T) {
		s := store.NewTaskStore()

		s.SetTasks([]taskserver.Task{
			{
				Name: "test",
			},
		})

		tasks := s.GetTasks()

		s.StartTaskRun(tasks[0].Name, []byte{}, nil)

		taskRuns := s.GetTaskRuns(tasks[0].ID)
		s.FailTaskRun(taskRuns[0].ID, "err")

		taskRuns = s.GetTaskRuns(tasks[0].ID)
		require.Equal(t, store.TaskRunStatusFailed, taskRuns[0].Status)
		require.Equal(t, "err", *taskRuns[0].Error)
	})
}

func TestGetTaskRunsByName(t *testing.T) {
	s := store.NewTaskStore()

	s.SetTasks([]taskserver.Task{
		{Name: "test"},
	})

	tasks := s.GetTasks()
	s.StartTaskRun(tasks[0].Name, []byte("input"), nil)

	t.Run("by task name", func(t *testing.T) {
		taskRuns := s.GetTaskRuns("test")
		require.Equal(t, 1, len(taskRuns))
		require.Equal(t, "test", taskRuns[0].TaskName)
	})

	t.Run("by task ID", func(t *testing.T) {
		taskRuns := s.GetTaskRuns(tasks[0].ID)
		require.Equal(t, 1, len(taskRuns))
		require.Equal(t, "test", taskRuns[0].TaskName)
	})

	t.Run("not found", func(t *testing.T) {
		taskRuns := s.GetTaskRuns("nonexistent")
		require.Equal(t, 0, len(taskRuns))
	})
}

func TestGetAllTaskRuns(t *testing.T) {
	s := store.NewTaskStore()

	s.SetTasks([]taskserver.Task{
		{Name: "task1"},
		{Name: "task2"},
	})

	s.StartTaskRun("task1", []byte("input1"), nil)
	s.StartTaskRun("task2", []byte("input2"), nil)
	s.StartTaskRun("task1", []byte("input3"), nil)

	taskRuns := s.GetAllTaskRuns()
	require.Equal(t, 3, len(taskRuns))
}

func TestGetTask(t *testing.T) {
	s := store.NewTaskStore()

	s.SetTasks([]taskserver.Task{
		{
			Name: "test",
		},
	})

	t.Run("by name", func(t *testing.T) {
		task := s.GetTask("test")
		require.NotNil(t, task)
		require.Equal(t, "test", task.Name)
	})

	t.Run("by ID", func(t *testing.T) {
		tasks := s.GetTasks()
		task := s.GetTask(tasks[0].ID)
		require.NotNil(t, task)
		require.Equal(t, "test", task.Name)
	})

	t.Run("by slug with workflow prefix", func(t *testing.T) {
		task := s.GetTask("my-workflow/test")
		require.NotNil(t, task)
		require.Equal(t, "test", task.Name)
	})

	t.Run("by slug with multi-segment prefix", func(t *testing.T) {
		task := s.GetTask("org/my-workflow/test")
		require.NotNil(t, task)
		require.Equal(t, "test", task.Name)
	})

	t.Run("not found", func(t *testing.T) {
		task := s.GetTask("nonexistent")
		require.Nil(t, task)
	})

	t.Run("slug not found", func(t *testing.T) {
		task := s.GetTask("my-workflow/nonexistent")
		require.Nil(t, task)
	})
}

// workflowRunIn starts a workflow run and moves it to status.
func workflowRunIn(t *testing.T, s *store.TaskStore, status store.TaskRunStatus) *store.TaskRun {
	t.Helper()
	s.SetTasks([]taskserver.Task{{Name: "wf", Workflow: pointers.From(true)}})
	run := s.StartTaskRun("wf", nil, nil)
	var err error
	switch status {
	case store.TaskRunStatusRunning:
	case store.TaskRunStatusPending:
		_, err = s.RecoverTaskRun(run.ID)
	case store.TaskRunStatusPaused:
		_, err = s.PauseTaskRun(run.ID)
	case store.TaskRunStatusComplete:
		_, err = s.CompleteTaskRun(run.ID, nil)
	case store.TaskRunStatusFailed:
		_, err = s.FailTaskRun(run.ID, "boom")
	case store.TaskRunStatusCanceled:
		_, err = s.CancelTaskRun(run.ID)
	default:
		t.Fatalf("unsupported status %q", status)
	}
	require.NoError(t, err)
	require.Equal(t, status, s.GetTaskRun(run.ID).Status)
	return run
}

func TestPauseTaskRun(t *testing.T) {
	tests := []struct {
		from    store.TaskRunStatus
		wantErr bool
	}{
		{from: store.TaskRunStatusRunning},
		{from: store.TaskRunStatusPending, wantErr: true},
		{from: store.TaskRunStatusPaused, wantErr: true},
		{from: store.TaskRunStatusComplete, wantErr: true},
		{from: store.TaskRunStatusFailed, wantErr: true},
		{from: store.TaskRunStatusCanceled, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(string(tt.from), func(t *testing.T) {
			s := store.NewTaskStore()
			run := workflowRunIn(t, s, tt.from)

			paused, err := s.PauseTaskRun(run.ID)

			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, tt.from, s.GetTaskRun(run.ID).Status)
				return
			}
			require.NoError(t, err)
			require.Equal(t, store.TaskRunStatusPaused, paused.Status)
			require.Equal(t, 0, paused.WorkflowAttempt, "pausing doesn't start an attempt")
		})
	}

	t.Run("unknown run", func(t *testing.T) {
		_, err := store.NewTaskStore().PauseTaskRun("trn-unknown")
		require.Error(t, err)
	})
}

func TestWakeTaskRun(t *testing.T) {
	tests := []struct {
		from    store.TaskRunStatus
		wantErr bool
	}{
		{from: store.TaskRunStatusPaused},
		{from: store.TaskRunStatusRunning, wantErr: true},
		{from: store.TaskRunStatusPending, wantErr: true},
		{from: store.TaskRunStatusComplete, wantErr: true},
		{from: store.TaskRunStatusFailed, wantErr: true},
		{from: store.TaskRunStatusCanceled, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(string(tt.from), func(t *testing.T) {
			s := store.NewTaskStore()
			run := workflowRunIn(t, s, tt.from)
			before := s.GetTaskRun(run.ID)

			woken, err := s.WakeTaskRun(run.ID)

			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, tt.from, s.GetTaskRun(run.ID).Status)
				return
			}
			require.NoError(t, err)
			require.Equal(t, store.TaskRunStatusPending, woken.Status)
			require.Equal(t, before.WorkflowAttempt+1, woken.WorkflowAttempt)
			require.Equal(t, before.Recoveries, woken.Recoveries, "a wake doesn't count against the recovery budget")

			_, err = s.WakeTaskRun(run.ID)
			require.Error(t, err, "a second wake is a no-op")
		})
	}
}

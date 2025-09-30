package store_test

import (
	"encoding/json"
	"testing"

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

func TestGetTask(t *testing.T) {
	s := store.NewTaskStore()

	s.SetTasks([]taskserver.Task{
		{
			Name: "test",
		},
	})

	task := s.GetTask("foo/test")
	require.Equal(t, "test", task.Name)
}

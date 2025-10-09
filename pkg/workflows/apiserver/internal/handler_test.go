package internal_test

import (
	"context"
	"encoding/json"
	"testing"

	workflowclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/workflows/apiserver/internal"
	"github.com/render-oss/cli/pkg/workflows/store"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
	"github.com/stretchr/testify/require"
)

func TestListTasks(t *testing.T) {
	store := store.NewTaskStore()

	store.SetTasks([]taskserver.Task{
		{
			Name: "test",
		},
	})

	got := internal.ListTasks(store)

	require.Equal(t, 1, len(got))
	require.Equal(t, "test", got[0].Task.Name)
}

func TestGetTask(t *testing.T) {
	store := store.NewTaskStore()

	store.SetTasks([]taskserver.Task{
		{
			Name: "test",
		},
	})

	t.Run("existing task", func(t *testing.T) {
		tasks := store.GetTasks()
		got := internal.GetTask(store, tasks[0].ID)
		require.Equal(t, "test", got.Name)
	})

	t.Run("non-existing task", func(t *testing.T) {
		got := internal.GetTask(store, "non-existing")
		require.Nil(t, got)
	})
}

type fakeRunner struct {
	startTask func(ctx context.Context, taskName string, input []byte, parentTaskRunID *string) (*store.TaskRun, error)
}

func (f *fakeRunner) StartTask(ctx context.Context, taskName string, input []byte, parentTaskRunID *string) (*store.TaskRun, error) {
	return f.startTask(ctx, taskName, input, parentTaskRunID)
}

func TestListTaskRuns(t *testing.T) {
	store := store.NewTaskStore()

	store.SetTasks([]taskserver.Task{
		{
			Name: "test",
		},
	})

	input := []byte("abc")

	store.StartTaskRun("test", input, nil)

	tasks := store.GetTasks()
	got := internal.ListTaskRuns(store, tasks[0].ID)

	require.Equal(t, 1, len(got))
	require.Equal(t, tasks[0].ID, got[0].TaskId)
}

func TestGetTaskRun(t *testing.T) {
	store := store.NewTaskStore()

	store.SetTasks([]taskserver.Task{
		{
			Name: "test",
		},
	})

	rawOutput := []interface{}{"def"}

	input, err := json.Marshal([]interface{}{"abc"})
	require.NoError(t, err)
	output, err := json.Marshal(rawOutput)
	require.NoError(t, err)

	store.StartTaskRun("test", input, nil)
	taskRuns := store.GetTaskRuns(store.GetTasks()[0].ID)
	store.CompleteTaskRun(taskRuns[0].ID, output)

	got := internal.GetTaskRun(store, taskRuns[0].ID)

	tasks := store.GetTasks()
	require.Equal(t, tasks[0].ID, got.TaskId)
	require.Equal(t, rawOutput, got.Results)
}

func TestGetTaskRunEvents(t *testing.T) {
	result, err := json.Marshal([]interface{}{"result"})
	require.NoError(t, err)

	t.Run("sends existing completed tasks", func(t *testing.T) {
		store := store.NewTaskStore()

		store.SetTasks([]taskserver.Task{
			{
				Name: "test",
			},
		})

		taskRun := store.StartTaskRun("test", []byte("test"), nil)
		store.CompleteTaskRun(taskRun.ID, result)

		ch, err := internal.GetTaskRunEvents(context.Background(), store, []string{taskRun.ID})
		require.NoError(t, err)

		result := <-ch
		require.Equal(t, workflowclient.Completed, result.Data.Status)
		require.NotNil(t, result.Data.StartedAt)
		require.NotNil(t, result.Data.CompletedAt)
		require.Equal(t, []interface{}{"result"}, result.Data.Results)
	})

	t.Run("sends new completed tasks", func(t *testing.T) {
		store := store.NewTaskStore()

		store.SetTasks([]taskserver.Task{
			{
				Name: "test",
			},
		})

		taskRun := store.StartTaskRun("test", []byte("test"), nil)

		ch, err := internal.GetTaskRunEvents(context.Background(), store, []string{taskRun.ID})
		require.NoError(t, err)

		go func() {
			store.CompleteTaskRun(taskRun.ID, result)
		}()

		tasks := store.GetTasks()

		result := <-ch
		require.Equal(t, tasks[0].ID, result.Data.TaskId)
		require.Equal(t, workflowclient.Completed, result.Data.Status)
		require.NotNil(t, result.Data.StartedAt)
		require.NotNil(t, result.Data.CompletedAt)
		require.Equal(t, []interface{}{"result"}, result.Data.Results)
	})
}

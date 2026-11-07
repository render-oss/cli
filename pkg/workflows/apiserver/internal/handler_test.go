package internal_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/render-oss/cli/pkg/client"
	workflowclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
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

func TestListTasksExcludesFlows(t *testing.T) {
	store := store.NewTaskStore()

	store.SetTasks([]taskserver.Task{
		{Name: "plain-task"},
		{Name: "my-flow", Workflow: pointers.From(true)},
	})

	got := internal.ListTasks(store)

	require.Len(t, got, 1)
	require.Equal(t, "plain-task", got[0].Task.Name)
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
	startTask func(ctx context.Context, taskName string, input []byte, parent *store.TaskRun) (*store.TaskRun, error)
}

func (f *fakeRunner) StartTask(ctx context.Context, taskName string, input []byte, parent *store.TaskRun) (*store.TaskRun, error) {
	return f.startTask(ctx, taskName, input, parent)
}

func TestMapTaskRunAttemptsNotNull(t *testing.T) {
	t.Run("running task run has one attempt", func(t *testing.T) {
		s := store.NewTaskStore()
		s.SetTasks([]taskserver.Task{{Name: "test"}})
		taskRun := s.StartTaskRun("test", []byte(`[1]`), nil)

		got := internal.MapTaskRun(s, taskRun)

		require.NotNil(t, got.Attempts)
		require.Len(t, got.Attempts, 1)
		require.Equal(t, workflowclient.Running, got.Attempts[0].Status)
		require.Nil(t, got.Attempts[0].CompletedAt)
	})

	t.Run("completed task run attempt has completedAt", func(t *testing.T) {
		s := store.NewTaskStore()
		s.SetTasks([]taskserver.Task{{Name: "test"}})
		taskRun := s.StartTaskRun("test", []byte(`[1]`), nil)
		s.CompleteTaskRun(taskRun.ID, []byte(`["done"]`))

		updated := s.GetTaskRun(taskRun.ID)
		got := internal.MapTaskRun(s, updated)

		require.Len(t, got.Attempts, 1)
		require.Equal(t, workflowclient.Completed, got.Attempts[0].Status)
		require.NotNil(t, got.Attempts[0].CompletedAt)
	})
}

func TestGetTaskRunDetailsAttempts(t *testing.T) {
	s := store.NewTaskStore()
	s.SetTasks([]taskserver.Task{{Name: "test"}})

	input, _ := json.Marshal([]interface{}{"abc"})
	output, _ := json.Marshal([]interface{}{"def"})

	taskRun := s.StartTaskRun("test", input, nil)
	s.CompleteTaskRun(taskRun.ID, output)

	got := internal.GetTaskRun(s, taskRun.ID)

	require.NotNil(t, got.Attempts)
	require.Len(t, got.Attempts, 1)
	require.Equal(t, workflowclient.Completed, got.Attempts[0].Status)
	require.NotNil(t, got.Attempts[0].CompletedAt)
	require.NotNil(t, got.Attempts[0].Results)
	require.Equal(t, []interface{}{"def"}, *got.Attempts[0].Results)
}

func TestListTaskRuns(t *testing.T) {
	t.Run("by task ID", func(t *testing.T) {
		store := store.NewTaskStore()

		store.SetTasks([]taskserver.Task{
			{Name: "test"},
		})

		store.StartTaskRun("test", []byte("abc"), nil)

		tasks := store.GetTasks()
		got := internal.ListTaskRuns(store, tasks[0].ID)

		require.Equal(t, 1, len(got))
		require.Equal(t, tasks[0].ID, got[0].TaskRun.TaskId)
	})

	t.Run("by task name", func(t *testing.T) {
		store := store.NewTaskStore()

		store.SetTasks([]taskserver.Task{
			{Name: "test"},
		})

		store.StartTaskRun("test", []byte("abc"), nil)

		tasks := store.GetTasks()
		got := internal.ListTaskRuns(store, "test")

		require.Equal(t, 1, len(got))
		require.Equal(t, tasks[0].ID, got[0].TaskRun.TaskId)
	})

	t.Run("all task runs when taskID is empty", func(t *testing.T) {
		store := store.NewTaskStore()

		store.SetTasks([]taskserver.Task{
			{Name: "task1"},
			{Name: "task2"},
		})

		store.StartTaskRun("task1", []byte("abc"), nil)
		store.StartTaskRun("task2", []byte("def"), nil)

		got := internal.ListTaskRuns(store, "")

		require.Equal(t, 2, len(got))
	})
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

func TestListFlows(t *testing.T) {
	s := store.NewTaskStore()
	s.SetTasks([]taskserver.Task{
		{Name: "plain-task"},
		{Name: "flow-a", Workflow: pointers.From(true)},
		{Name: "flow-b", Workflow: pointers.From(true)},
	})

	names := func(flows []*client.FlowWithCursor) []string {
		got := make([]string, 0, len(flows))
		for _, f := range flows {
			got = append(got, f.Flow.Name)
		}
		return got
	}

	t.Run("lists only workflow tasks", func(t *testing.T) {
		got := internal.ListFlows(s, nil, 0, "")
		require.ElementsMatch(t, []string{"flow-a", "flow-b"}, names(got))
		for _, f := range got {
			require.Equal(t, f.Flow.Id, f.Cursor)
		}
	})

	t.Run("matches slugs on the flow name", func(t *testing.T) {
		got := internal.ListFlows(s, []string{"my-workflow/flow-a:3", "plain-task"}, 0, "")
		require.Equal(t, []string{"flow-a"}, names(got))
	})

	t.Run("pages by limit and cursor", func(t *testing.T) {
		first := internal.ListFlows(s, nil, 1, "")
		require.Len(t, first, 1)

		second := internal.ListFlows(s, nil, 1, string(first[0].Cursor))
		require.Len(t, second, 1)
		require.NotEqual(t, first[0].Flow.Id, second[0].Flow.Id)

		require.Empty(t, internal.ListFlows(s, nil, 1, string(second[0].Cursor)))
	})
}

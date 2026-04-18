package internal

import (
	"context"
	"slices"
	"sort"

	"github.com/render-oss/cli/v2/pkg/client"
	logclient "github.com/render-oss/cli/v2/pkg/client/logs"
	workflowclient "github.com/render-oss/cli/v2/pkg/client/workflows"
	"github.com/render-oss/cli/v2/pkg/pointers"
	"github.com/render-oss/cli/v2/pkg/workflows/apiserver/internal/serversideevents"
	"github.com/render-oss/cli/v2/pkg/workflows/logs"
	"github.com/render-oss/cli/v2/pkg/workflows/store"
)

func ListTasks(store *store.TaskStore) []*client.TaskWithCursor {
	tasks := store.GetTasks()

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})

	taskList := make([]*client.TaskWithCursor, len(tasks))
	for i, task := range tasks {
		taskList[i] = &client.TaskWithCursor{
			Task:   *mapTask(task),
			Cursor: task.ID,
		}
	}

	return taskList
}

func GetTask(store *store.TaskStore, taskID string) *workflowclient.Task {
	task := store.GetTask(taskID)

	if task == nil {
		return nil
	}

	return mapTask(task)
}

func ListTaskRuns(s *store.TaskStore, taskNameOrID string) []*client.TaskRunWithCursor {
	var taskRuns []*store.TaskRun
	if taskNameOrID == "" {
		taskRuns = s.GetAllTaskRuns()
	} else {
		taskRuns = s.GetTaskRuns(taskNameOrID)
	}

	taskRunList := make([]*client.TaskRunWithCursor, len(taskRuns))
	for i, taskRun := range taskRuns {
		mapped := MapTaskRun(s, taskRun)
		taskRunList[i] = &client.TaskRunWithCursor{
			TaskRun: *mapped,
			Cursor:  taskRun.ID,
		}
	}

	return taskRunList
}

func GetTaskRun(store *store.TaskStore, taskRunID string) *workflowclient.TaskRunDetails {
	taskRun := store.GetTaskRun(taskRunID)

	return mapTaskRunDetails(store, taskRun)
}

func ListLogs(logStore *logs.LogStore, input client.ListLogsParams) []logclient.Log {
	searchParams := MapLogSearchParams(input)
	logs := logStore.GetLogs(searchParams)
	return mapLogs(logs)
}

func sendResultToChannel(s *store.TaskStore, taskRun *store.TaskRun, outputCh chan serversideevents.Message[workflowclient.TaskRunDetails], taskRunIDs []string) {
	taskRunDetails := mapTaskRunDetails(s, taskRun)
	if taskRunDetails == nil || !slices.Contains(taskRunIDs, taskRunDetails.Id) {
		return
	}
	outputCh <- serversideevents.Message[workflowclient.TaskRunDetails]{
		Event: pointers.From("task.completed"),
		Data:  *taskRunDetails,
	}
}

func GetTaskRunEvents(ctx context.Context, s *store.TaskStore, taskRunIDs []string) (chan serversideevents.Message[workflowclient.TaskRunDetails], error) {
	ch := make(chan *store.TaskRun)
	s.AddTaskRunChan(ch)

	outputCh := make(chan serversideevents.Message[workflowclient.TaskRunDetails])

	go func() {
		defer func() {
			s.RemoveTaskRunChan(ch)
			close(ch)
		}()

		// Send existing task run events
		for _, taskRunID := range taskRunIDs {
			taskRun := s.GetTaskRun(taskRunID)
			if taskRun == nil || (taskRun.Status != store.TaskRunStatusComplete && taskRun.Status != store.TaskRunStatusFailed) {
				continue
			}
			sendResultToChannel(s, taskRun, outputCh, taskRunIDs)
		}

		// Send subsequent task run events
		for {
			select {
			case <-ctx.Done():
				return
			case taskRun := <-ch:
				sendResultToChannel(s, taskRun, outputCh, taskRunIDs)
			}
		}
	}()

	return outputCh, nil
}

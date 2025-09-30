package internal

import (
	"context"
	"slices"

	"github.com/render-oss/cli/pkg/client"
	logclient "github.com/render-oss/cli/pkg/client/logs"
	workflowclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/workflows/apiserver/internal/serversideevents"
	"github.com/render-oss/cli/pkg/workflows/logs"
	"github.com/render-oss/cli/pkg/workflows/store"
)

func ListTasks(store *store.TaskStore) []*workflowclient.Task {
	tasks := store.GetTasks()

	taskList := make([]*workflowclient.Task, len(tasks))
	for i, task := range tasks {
		taskList[i] = mapTask(task)
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

func ListTaskRuns(store *store.TaskStore, taskID string) []*workflowclient.TaskRun {
	taskRuns := store.GetTaskRuns(taskID)

	taskRunList := make([]*workflowclient.TaskRun, len(taskRuns))
	for i, taskRun := range taskRuns {
		taskRunList[i] = MapTaskRun(store, taskRun)
	}

	return taskRunList
}

func GetTaskRun(store *store.TaskStore, taskRunID string) *workflowclient.TaskRunDetails {
	taskRun := store.GetTaskRun(taskRunID)

	return mapTaskRunDetails(store, taskRun)
}

func ListLogs(logStore *logs.LogStore, input client.ListLogsParams) []*logclient.Log {
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

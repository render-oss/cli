package internal

import (
	"context"
	"slices"
	"sort"
	"strings"

	"github.com/render-oss/cli/pkg/client"
	logclient "github.com/render-oss/cli/pkg/client/logs"
	workflowclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/workflows/apiserver/internal/serversideevents"
	"github.com/render-oss/cli/pkg/workflows/logs"
	"github.com/render-oss/cli/pkg/workflows/store"
)

// ListTasks returns the registered tasks, sorted by ID. Tasks registered with
// workflow() are flows and are listed by ListFlows instead.
func ListTasks(store *store.TaskStore) []*client.TaskWithCursor {
	tasks := store.GetTasks()

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})

	taskList := make([]*client.TaskWithCursor, 0, len(tasks))
	for _, task := range tasks {
		if task.IsWorkflow {
			continue
		}
		taskList = append(taskList, &client.TaskWithCursor{
			Task:   *mapTask(task),
			Cursor: task.ID,
		})
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

// ListFlows returns the tasks registered with workflow(), sorted by ID. A
// flow slug matches on its flow name: local dev has no workflow slug or
// versions, so "workflow-slug/" prefixes and ":version" suffixes are ignored.
// The cursor is the last ID of the previous page; a limit of 0 returns all.
func ListFlows(store *store.TaskStore, flowSlugs []string, limit int, cursor string) []*client.FlowWithCursor {
	names := make([]string, 0, len(flowSlugs))
	for _, slug := range flowSlugs {
		if idx := strings.LastIndex(slug, "/"); idx != -1 {
			slug = slug[idx+1:]
		}
		if idx := strings.Index(slug, ":"); idx != -1 {
			slug = slug[:idx]
		}
		names = append(names, slug)
	}

	tasks := store.GetTasks()
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})

	flowList := make([]*client.FlowWithCursor, 0)
	for _, task := range tasks {
		if !task.IsWorkflow || task.ID <= cursor {
			continue
		}
		if len(names) > 0 && !slices.Contains(names, task.Name) {
			continue
		}
		if limit > 0 && len(flowList) == limit {
			break
		}
		flowList = append(flowList, &client.FlowWithCursor{
			Flow:   *mapFlow(task),
			Cursor: task.ID,
		})
	}

	return flowList
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

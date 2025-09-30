package internal

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/render-oss/cli/pkg/client"
	logClient "github.com/render-oss/cli/pkg/client/logs"
	workflowClient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/workflows/logs"
	"github.com/render-oss/cli/pkg/workflows/store"
)

func mapTask(task *store.Task) *workflowClient.Task {
	return &workflowClient.Task{
		Id:        task.ID,
		Name:      task.Name,
		CreatedAt: task.CreatedAt,
	}
}

func mapTaskRunStatus(status store.TaskRunStatus) workflowClient.TaskRunStatus {
	switch status {
	case store.TaskRunStatusRunning:
		return workflowClient.Running
	case store.TaskRunStatusComplete:
		return workflowClient.Completed
	case store.TaskRunStatusFailed:
		return workflowClient.Failed
	default:
		return workflowClient.Running
	}
}

func MapTaskRun(store *store.TaskStore, taskRun *store.TaskRun) *workflowClient.TaskRun {
	task := store.GetTaskByName(taskRun.TaskName)

	return &workflowClient.TaskRun{
		Id:          taskRun.ID,
		TaskId:      task.ID,
		Status:      mapTaskRunStatus(taskRun.Status),
		StartedAt:   taskRun.StartedAt,
		CompletedAt: taskRun.CompletedAt,
	}
}

func mapTaskRunDetails(store *store.TaskStore, taskRun *store.TaskRun) *workflowClient.TaskRunDetails {
	task := store.GetTaskByName(taskRun.TaskName)

	var results workflowClient.TaskRunResult
	// ignore error, we will just return empty results
	_ = json.Unmarshal(taskRun.Output, &results)

	return &workflowClient.TaskRunDetails{
		Id:          taskRun.ID,
		TaskId:      task.ID,
		Status:      mapTaskRunStatus(taskRun.Status),
		StartedAt:   taskRun.StartedAt,
		CompletedAt: taskRun.CompletedAt,
		Results:     results,
		Error:       taskRun.Error,
	}
}

func ParseLogSearchQueryParams(r *http.Request) (client.ListLogsParams, error) {
	var params client.ListLogsParams
	if r.URL.Query().Get("taskRunID") != "" {
		taskRunIDs := strings.Split(r.URL.Query().Get("taskRunID"), ",")
		params.TaskRun = pointers.From(taskRunIDs)
	}

	if r.URL.Query().Get("startTime") != "" {
		startTime, err := time.Parse(time.RFC3339, r.URL.Query().Get("startTime"))
		if err != nil {
			return client.ListLogsParams{}, err
		}
		params.StartTime = &startTime
	}

	if r.URL.Query().Get("endTime") != "" {
		endTime, err := time.Parse(time.RFC3339, r.URL.Query().Get("endTime"))
		if err != nil {
			return client.ListLogsParams{}, err
		}
		params.EndTime = &endTime
	}

	if r.URL.Query().Get("text") != "" {
		texts := strings.Split(r.URL.Query().Get("text"), ",")
		params.Text = pointers.From(texts)
	}

	return params, nil
}

func MapLogSearchParams(params client.ListLogsParams) logs.LogSearch {
	var taskRunIDs []string
	if params.TaskRun != nil {
		taskRunIDs = *params.TaskRun
	}

	var text []string
	if params.Text != nil {
		text = *params.Text
	}

	var startTime time.Time
	if params.StartTime != nil {
		startTime = *params.StartTime
	}

	var endTime time.Time
	if params.EndTime != nil {
		endTime = *params.EndTime
	}

	return logs.LogSearch{
		TaskRunID: taskRunIDs,
		Text:      text,
		StartTime: startTime,
		EndTime:   endTime,
	}
}

func mapLogs(logs logs.Logs) []*logClient.Log {
	logsList := make([]*logClient.Log, len(logs))
	for i, log := range logs {
		logsList[i] = mapLog(log)
	}
	return logsList
}

func mapLog(log *logs.Log) *logClient.Log {
	return &logClient.Log{
		Id:        log.ID,
		Message:   log.Message,
		Timestamp: log.Timestamp,
	}
}

func ForwardLogsToWebsocket(ch <-chan *logs.Log, readCh <-chan WebSocketData, writeCh chan<- WebSocketData) {
	for {
		select {
		// Forward logs to the client.
		case log, ok := <-ch:
			// If the channel is done sending return
			if !ok {
				return
			}

			jsonLog, err := json.Marshal(mapLog(log))
			if err != nil {
				writeCh <- WebSocketData{
					MessageType: websocket.CloseAbnormalClosure,
					Data:        websocket.FormatCloseMessage(websocket.CloseAbnormalClosure, "unable to forward log"),
				}
			}

			// Send the log message to the client.
			//
			// If there's an error, bail.
			writeCh <- WebSocketData{
				MessageType: websocket.TextMessage,
				Data:        jsonLog,
			}

		// Bail on client close.
		case _, more := <-readCh:
			if !more {
				return
			}
		}
	}
}

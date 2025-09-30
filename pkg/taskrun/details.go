package taskrun

import (
	"encoding/json"
	"time"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/tui"
)

func TaskRunDetailsFormat(taskRun *wfclient.TaskRunDetails) []tui.KeyValue {
	if taskRun == nil {
		return nil
	}

	startedAt := ""
	if taskRun.StartedAt != nil {
		startedAt = taskRun.StartedAt.Format(time.RFC3339)
	}

	completedAt := ""
	if taskRun.CompletedAt != nil {
		completedAt = taskRun.CompletedAt.Format(time.RFC3339)
	}

	keyValues := []tui.KeyValue{
		{Key: "ID", Value: taskRun.Id},
		{Key: "Status", Value: statusWithStyle(taskRun.Status).Render(string(taskRun.Status))},
		{Key: "Started At", Value: startedAt},
		{Key: "Completed At", Value: completedAt},
	}

	if taskRun.Error != nil {
		keyValues = append(keyValues, tui.KeyValue{Key: "Error", Value: *taskRun.Error})
	}

	if taskRun.Results != nil {
		results, err := json.Marshal(taskRun.Results)
		if err != nil {
			keyValues = append(keyValues, tui.KeyValue{Key: "Error", Value: err.Error()})
		} else {
			keyValues = append(keyValues, tui.KeyValue{Key: "Results", Value: string(results)})
		}
	}

	return keyValues
}

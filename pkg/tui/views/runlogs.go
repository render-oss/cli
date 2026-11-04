package views

import (
	"time"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/pointers"
)

// RunLogInput returns the log query for one task or flow run. Both kinds of
// run tag their logs with the run's ID in the task run field. A finished run
// shows its logs from just before it started to just after it completed; an
// unfinished run tails.
func RunLogInput(workflowID, runID string, startedAt, completedAt *time.Time, finished bool) LogInput {
	var startTime *command.TimeOrRelative
	if startedAt != nil {
		t := startedAt.Add(-5 * time.Second)
		startTime = &command.TimeOrRelative{T: &t}
	}
	var endTime *command.TimeOrRelative
	if completedAt != nil {
		t := completedAt.Add(5 * time.Second)
		endTime = &command.TimeOrRelative{T: &t}
	}

	return LogInput{
		ResourceIDs: []string{workflowID},
		TaskRunID:   []string{runID},
		StartTime:   startTime,
		EndTime:     endTime,
		Tail:        !finished,
	}
}

// StartedRunLogInput tails the logs of a run that was just started.
func StartedRunLogInput(workflowID, runID string) LogInput {
	return LogInput{
		// Start querying logs from 10 seconds ago to avoid missing any logs
		StartTime:   &command.TimeOrRelative{T: pointers.From(time.Now().Add(-10 * time.Second))},
		ResourceIDs: []string{workflowID},
		TaskRunID:   []string{runID},
		Tail:        true,
	}
}

package orchestrator_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/workflows/orchestrator"
	"github.com/render-oss/cli/pkg/workflows/store"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
)

func TestPrintStatusReporterSteps(t *testing.T) {
	var lines []string
	reporter := orchestrator.NewPrintStatusReporter(func(format string, args ...any) {
		lines = append(lines, fmt.Sprintf(format, args...))
	}, orchestrator.WithStatusReporterTimestamps(false), orchestrator.WithStatusReporterIncludeInputs(true))

	run := &store.TaskRun{ID: "trn-1", TaskName: "checkout", IsWorkflow: true}
	startedAt := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	step := taskserver.StepRecord{
		StepKey:   taskserver.StepKey{Seq: 2},
		TaskName:  pointers.From("charge"),
		Input:     pointers.From([]byte(`[ "card" ]`)),
		StartedAt: startedAt,
	}

	reporter.TaskRunning(run)
	reporter.StepStarted(run, step)
	reporter.StepRetrying(run, step, taskserver.StepAttemptRecord{
		StepKey: step.StepKey, Attempt: 0, Error: taskserver.StepError{Name: "TimeoutError", Message: "timed out"},
	})

	completed := step
	completed.CompletedAt = pointers.From(startedAt.Add(1500 * time.Millisecond))
	completed.Output = pointers.From([]byte(`{"ok": true}`))
	reporter.StepCompleted(run, completed)

	failed := step
	failed.CompletedAt = pointers.From(startedAt.Add(20 * time.Millisecond))
	failed.Error = &taskserver.StepError{Name: "Error", Message: "card declined"}
	reporter.StepCompleted(run, failed)

	require.Equal(t, []string{
		`Flow running: trn-1 (checkout) input=[]`,
		`  ↳ Step Running [2 (charge)]: input=["card"]`,
		`  ↳ Step Retrying [2 (charge) attempt=0]: TimeoutError: timed out`,
		`  ↳ Step Completed [2 (charge)]: output={"ok":true} duration=1.5s`,
		`  ↳ Step Failed [2 (charge)]: Error: card declined duration=20ms`,
	}, lines)
}

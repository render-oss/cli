package orchestrator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/render-oss/cli/pkg/pointers"
	renderstyle "github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/workflows/store"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
)

type PrintFunc func(format string, args ...any)

type PrintStatusReporterOption func(*PrintStatusReporter)

func NewPrintStatusReporter(printFn PrintFunc, opts ...PrintStatusReporterOption) *PrintStatusReporter {
	reporter := &PrintStatusReporter{
		printFn:        printFn,
		withTimestamps: true,
		showEnqueued:   true,
		includeInputs:  false,
	}
	for _, opt := range opts {
		opt(reporter)
	}
	return reporter
}

type PrintStatusReporter struct {
	printFn PrintFunc

	withTimestamps bool
	showEnqueued   bool
	includeInputs  bool
	runDepth       map[string]int
	parentTaskName map[string]string
	depthMu        sync.Mutex
}

func WithStatusReporterTimestamps(enabled bool) PrintStatusReporterOption {
	return func(r *PrintStatusReporter) {
		r.withTimestamps = enabled
	}
}

func WithStatusReporterTaskEnqueued(enabled bool) PrintStatusReporterOption {
	return func(r *PrintStatusReporter) {
		r.showEnqueued = enabled
	}
}

func WithStatusReporterIncludeInputs(enabled bool) PrintStatusReporterOption {
	return func(r *PrintStatusReporter) {
		r.includeInputs = enabled
	}
}

func (r *PrintStatusReporter) TaskEnqueued(taskRun *store.TaskRun) {
	if !r.showEnqueued {
		return
	}
	r.print("%s enqueued: %s", r.taskLabel(taskRun), r.describeTaskRun(taskRun))
}

func (r *PrintStatusReporter) TaskRunning(taskRun *store.TaskRun) {
	r.print("%s running: %s", r.taskLabel(taskRun), r.describeTaskRun(taskRun))
}

func (r *PrintStatusReporter) TaskCompleted(taskRun *store.TaskRun) {
	status := renderstyle.Status.Foreground(renderstyle.ColorOK).Render("Completed")
	r.print(r.formatWithDuration("%s %s: %s output=%s", taskRun), r.taskLabel(taskRun), status, r.describeTaskRun(taskRun), formatTaskRunOutput(taskRun))
}

func (r *PrintStatusReporter) TaskFailed(taskRun *store.TaskRun) {
	details := ""
	if taskRun != nil && taskRun.Error != nil {
		if trimmed := strings.TrimSpace(*taskRun.Error); trimmed != "" {
			details = fmt.Sprintf(" - %s", trimmed)
		}
	}
	status := renderstyle.Status.Foreground(renderstyle.ColorError).Render("Failed")
	r.print(r.formatWithDuration("%s %s: %s%s", taskRun), r.taskLabel(taskRun), status, r.describeTaskRun(taskRun), details)
}

func (r *PrintStatusReporter) TaskCancelled(taskRun *store.TaskRun) {
	status := renderstyle.Status.Foreground(renderstyle.ColorWarning).Render("Cancelled")
	r.print(r.formatWithDuration("%s %s: %s", taskRun), r.taskLabel(taskRun), status, r.describeTaskRun(taskRun))
}

// TaskPaused reports a workflow run whose attempt suspended to wait on a
// child it invoked. reason says what it's waiting for.
func (r *PrintStatusReporter) TaskPaused(taskRun *store.TaskRun, reason string) {
	status := renderstyle.Status.Foreground(renderstyle.ColorWarning).Render("Paused")
	r.print("%s %s: %s %s", r.taskLabel(taskRun), status, r.describeTaskRun(taskRun), reason)
}

// TaskReplaying reports a workflow attempt that crashed and is being replayed.
// It's a log line, not a status: the run is pending until the next attempt
// starts, as on the platform.
func (r *PrintStatusReporter) TaskReplaying(taskRun *store.TaskRun) {
	status := renderstyle.Status.Foreground(renderstyle.ColorWarning).Render("Replaying")
	r.print("%s %s: %s attempt=%d", r.taskLabel(taskRun), status, r.describeTaskRun(taskRun), taskRun.WorkflowAttempt)
}

// Step lines share one shape so they're easy to scan:
//
//	↳ Step <Action> [<seq> (<name>) <attempt>]: <input, output, or error>

// StepStarted reports an inline step of a workflow run starting.
func (r *PrintStatusReporter) StepStarted(taskRun *store.TaskRun, step taskserver.StepRecord) {
	detail := ""
	if r.includeInputs {
		detail = fmt.Sprintf(": input=%s", formatPayload(step.Input))
	}
	r.print("%s Running [%s]%s", r.stepLabel(taskRun), formatStepDescriptor(step), detail)
}

// StepRetrying reports a failed attempt of an inline step that the SDK will
// retry.
func (r *PrintStatusReporter) StepRetrying(taskRun *store.TaskRun, step taskserver.StepRecord, attempt taskserver.StepAttemptRecord) {
	status := renderstyle.Status.Foreground(renderstyle.ColorWarning).Render("Retrying")
	r.print("%s %s [%s attempt=%d]: %s", r.stepLabel(taskRun), status, formatStepDescriptor(step), attempt.Attempt, formatStepError(&attempt.Error))
}

// StepCompleted reports an inline step finishing with an output or an error.
func (r *PrintStatusReporter) StepCompleted(taskRun *store.TaskRun, step taskserver.StepRecord) {
	duration := ""
	if d := formatDuration(&step.StartedAt, step.CompletedAt); d != "" {
		duration = fmt.Sprintf(" duration=%s", d)
	}
	if step.Error != nil {
		status := renderstyle.Status.Foreground(renderstyle.ColorError).Render("Failed")
		r.print("%s %s [%s]: %s%s", r.stepLabel(taskRun), status, formatStepDescriptor(step), formatStepError(step.Error), duration)
		return
	}
	status := renderstyle.Status.Foreground(renderstyle.ColorOK).Render("Completed")
	r.print("%s %s [%s]: output=%s%s", r.stepLabel(taskRun), status, formatStepDescriptor(step), formatPayload(step.Output), duration)
}

func (r *PrintStatusReporter) TaskNotFound(taskSlug string) {
	status := renderstyle.Status.Foreground(renderstyle.ColorError).Render("Not found")
	r.print("Task %s: %s", status, taskSlug)
}

func (r *PrintStatusReporter) print(format string, args ...any) {
	if r.printFn == nil {
		return
	}
	if r.withTimestamps {
		params := append([]any{time.Now().Format("15:04:05")}, args...)
		r.printFn("[%s] "+format, params...)
		return
	}
	r.printFn(format, args...)
}

func (r *PrintStatusReporter) describeTaskRun(taskRun *store.TaskRun) string {
	desc := formatTaskRunDescriptor(taskRun)
	if r.includeInputs {
		desc = fmt.Sprintf("%s input=%s", desc, formatTaskRunInput(taskRun))
	}
	return desc
}

func (r *PrintStatusReporter) taskLabel(taskRun *store.TaskRun) string {
	depth := r.depthFor(taskRun)
	indent := strings.Repeat("  ", depth)
	if depth == 0 {
		if taskRun != nil && taskRun.IsWorkflow {
			return fmt.Sprintf("%sFlow", indent)
		}
		return fmt.Sprintf("%sTask", indent)
	}
	if parentName := r.parentNameFor(taskRun); parentName != "" {
		return fmt.Sprintf("%s↳ Subtask (parent: %s)", indent, parentName)
	}
	return fmt.Sprintf("%s↳ Subtask", indent)
}

// stepLabel labels a step one level below its workflow run.
func (r *PrintStatusReporter) stepLabel(taskRun *store.TaskRun) string {
	return fmt.Sprintf("%s↳ Step", strings.Repeat("  ", r.depthFor(taskRun)+1))
}

func (r *PrintStatusReporter) depthFor(taskRun *store.TaskRun) int {
	if taskRun == nil {
		return 0
	}
	r.depthMu.Lock()
	defer r.depthMu.Unlock()
	if r.runDepth == nil {
		r.runDepth = make(map[string]int)
	}
	if r.parentTaskName == nil {
		r.parentTaskName = make(map[string]string)
	}

	// Always store the task name for this task run so children can look it up
	if _, ok := r.parentTaskName[taskRun.ID]; !ok {
		r.parentTaskName[taskRun.ID] = taskRun.TaskName
	}

	if depth, ok := r.runDepth[taskRun.ID]; ok {
		return depth
	}

	depth := 0
	if taskRun.ParentTaskRunID != nil {
		if parentDepth, ok := r.runDepth[*taskRun.ParentTaskRunID]; ok {
			depth = parentDepth + 1
		} else {
			depth = 1
		}
	}
	r.runDepth[taskRun.ID] = depth
	return depth
}

func (r *PrintStatusReporter) parentNameFor(taskRun *store.TaskRun) string {
	if taskRun == nil || taskRun.ParentTaskRunID == nil {
		return ""
	}
	r.depthMu.Lock()
	defer r.depthMu.Unlock()
	if r.parentTaskName == nil {
		return ""
	}
	return r.parentTaskName[*taskRun.ParentTaskRunID]
}

func formatTaskRunDescriptor(taskRun *store.TaskRun) string {
	if taskRun == nil {
		return "(unknown task run)"
	}
	if taskRun.TaskName == "" {
		return taskRun.ID
	}
	return fmt.Sprintf("%s (%s)", taskRun.ID, taskRun.TaskName)
}

func formatStepDescriptor(step taskserver.StepRecord) string {
	name := pointers.StringValue(step.TaskName)
	if name == "" {
		return formatStepKey(step.StepKey)
	}
	return fmt.Sprintf("%s (%s)", formatStepKey(step.StepKey), name)
}

func formatStepError(stepErr *taskserver.StepError) string {
	if stepErr == nil {
		return ""
	}
	message := strings.TrimSpace(stepErr.Message)
	if stepErr.Name != "" {
		message = fmt.Sprintf("%s: %s", stepErr.Name, message)
	}
	return message
}

func formatTaskRunInput(taskRun *store.TaskRun) string {
	if taskRun == nil || len(taskRun.Input) == 0 {
		return "[]"
	}

	var buf bytes.Buffer
	if err := json.Compact(&buf, taskRun.Input); err == nil {
		return buf.String()
	}
	return string(taskRun.Input)
}

const maxOutputDisplayLen = 500

func formatTaskRunOutput(taskRun *store.TaskRun) string {
	if taskRun == nil {
		return "null"
	}
	return formatPayload(pointers.From([]byte(taskRun.Output)))
}

// formatPayload compacts JSON for display, truncating long values.
func formatPayload(data *[]byte) string {
	if data == nil || len(*data) == 0 {
		return "null"
	}

	var buf bytes.Buffer
	if err := json.Compact(&buf, *data); err != nil {
		buf.Reset()
		buf.Write(*data)
	}

	s := buf.String()
	if len(s) > maxOutputDisplayLen {
		return s[:maxOutputDisplayLen] + "... (truncated)"
	}
	return s
}

func (r *PrintStatusReporter) formatWithDuration(format string, taskRun *store.TaskRun) string {
	if duration := formatTaskRunDuration(taskRun); duration != "" {
		return format + fmt.Sprintf(" duration=%s", duration)
	}
	return format
}

func formatTaskRunDuration(taskRun *store.TaskRun) string {
	if taskRun == nil {
		return ""
	}
	return formatDuration(taskRun.StartedAt, taskRun.CompletedAt)
}

// formatDuration formats the time from start to end, or to now while end is
// unset.
func formatDuration(start, end *time.Time) string {
	if start == nil {
		return ""
	}

	stop := time.Now()
	if end != nil {
		stop = *end
	}
	duration := stop.Sub(*start)
	if duration < 0 {
		return ""
	}

	if duration >= time.Second {
		duration = duration.Round(10 * time.Millisecond)
	} else {
		duration = duration.Round(time.Millisecond)
	}

	return duration.String()
}

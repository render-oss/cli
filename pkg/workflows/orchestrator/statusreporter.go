package orchestrator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	renderstyle "github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/workflows/store"
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
	r.print(r.formatWithDuration("%s %s: %s", taskRun), r.taskLabel(taskRun), status, r.describeTaskRun(taskRun))
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
		return fmt.Sprintf("%sTask", indent)
	}
	if parentName := r.parentNameFor(taskRun); parentName != "" {
		return fmt.Sprintf("%s↳ Subtask (parent: %s)", indent, parentName)
	}
	return fmt.Sprintf("%s↳ Subtask", indent)
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

func (r *PrintStatusReporter) formatWithDuration(format string, taskRun *store.TaskRun) string {
	if duration := formatTaskRunDuration(taskRun); duration != "" {
		return format + fmt.Sprintf(" duration=%s", duration)
	}
	return format
}

func formatTaskRunDuration(taskRun *store.TaskRun) string {
	if taskRun == nil || taskRun.StartedAt == nil {
		return ""
	}

	end := time.Now()
	if taskRun.CompletedAt != nil {
		end = *taskRun.CompletedAt
	}
	duration := end.Sub(*taskRun.StartedAt)
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
